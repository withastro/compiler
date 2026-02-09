//! Astro code printer.
//!
//! Transforms an `AstroRoot` AST into JavaScript code compatible with the Astro runtime.

use cow_utils::CowUtils;
use oxc_allocator::Allocator;
use oxc_ast::ast::*;
use oxc_data_structures::code_buffer::CodeBuffer;
use rustc_hash::FxHashMap;

use oxc_codegen::{Codegen, Context, Gen, GenExpr};

use crate::TransformOptions;
use crate::scanner::{
    AstroScanner, HoistedScript, HoistedScriptType as InternalHoistedScriptType, HydratedComponent,
    ScanResult, get_jsx_attribute_name, get_jsx_element_name, is_component_name, is_custom_element,
    should_hoist_script,
};

/// Runtime function names used in generated code.
mod runtime {
    pub const FRAGMENT: &str = "Fragment";
    pub const RENDER: &str = "$$render";
    pub const CREATE_ASTRO: &str = "$$createAstro";
    pub const CREATE_COMPONENT: &str = "$$createComponent";
    pub const RENDER_COMPONENT: &str = "$$renderComponent";
    pub const RENDER_HEAD: &str = "$$renderHead";
    pub const MAYBE_RENDER_HEAD: &str = "$$maybeRenderHead";
    pub const UNESCAPE_HTML: &str = "$$unescapeHTML";
    pub const RENDER_SLOT: &str = "$$renderSlot";
    pub const MERGE_SLOTS: &str = "$$mergeSlots";
    pub const ADD_ATTRIBUTE: &str = "$$addAttribute";
    pub const SPREAD_ATTRIBUTES: &str = "$$spreadAttributes";
    pub const DEFINE_STYLE_VARS: &str = "$$defineStyleVars";
    pub const DEFINE_SCRIPT_VARS: &str = "$$defineScriptVars";
    pub const RENDER_TRANSITION: &str = "$$renderTransition";
    pub const CREATE_TRANSITION_SCOPE: &str = "$$createTransitionScope";
    pub const RENDER_SCRIPT: &str = "$$renderScript";
    pub const CREATE_METADATA: &str = "$$createMetadata";
    pub const RESULT: &str = "$$result";
}

/// The type of a hoisted script.
///
/// Matches Go compiler's `HoistedScript.type` field.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum HoistedScriptType {
    /// An inline script with code content.
    Inline,
    /// An external script with a `src` URL.
    External,
}

/// A hoisted script extracted from the Astro template.
///
/// Matches Go compiler's `HoistedScript` shape.
#[derive(Debug, Clone)]
pub struct TransformResultHoistedScript {
    /// The script type: `"inline"` or `"external"`.
    pub script_type: HoistedScriptType,
    /// The inline script code (when `script_type` is `Inline`).
    pub code: Option<String>,
    /// The external script src URL (when `script_type` is `External`).
    pub src: Option<String>,
    // Note: the Go compiler also has a `map` field for inline scripts (sourcemap),
    // which we stub as empty string for now.
}

/// A hydrated component reference found in the template.
///
/// Matches Go compiler's `HydratedComponent` shape.
#[derive(Debug, Clone)]
pub struct TransformResultHydratedComponent {
    /// The export name from the module (e.g., `"default"` or a named export).
    pub export_name: String,
    /// The local variable name used in the component.
    pub local_name: String,
    /// The import specifier (e.g., `"../components/Counter.jsx"`).
    pub specifier: String,
    /// The resolved path (filled by `resolvePath` callback; empty string if unresolved).
    pub resolved_path: String,
}

/// Output from Astro code generation.
///
/// Matches Go compiler's `TransformResult` shape from `@astrojs/compiler`.
#[derive(Debug)]
pub struct TransformResult {
    /// The generated JavaScript code.
    pub code: String,
    /// Source map JSON string (stub: empty string until sourcemap support is implemented).
    pub map: String,
    /// CSS scope hash for the component (e.g., `"astro-XXXXXX"`).
    pub scope: String,
    /// Extracted CSS strings from `<style>` tags (stub: empty vec until CSS support).
    pub style_error: Vec<String>,
    /// Diagnostic messages (stub: empty vec).
    pub diagnostics: Vec<String>,
    /// Extracted CSS from `<style>` tags (stub: empty vec until CSS support).
    pub css: Vec<String>,
    /// Hoisted scripts extracted from the template.
    pub scripts: Vec<TransformResultHoistedScript>,
    /// Components with `client:*` hydration directives (except `client:only`).
    pub hydrated_components: Vec<TransformResultHydratedComponent>,
    /// Components with `client:only` directive.
    pub client_only_components: Vec<TransformResultHydratedComponent>,
    /// Components with `server:defer` directive.
    pub server_components: Vec<TransformResultHydratedComponent>,
    /// Whether the template contains an explicit `<head>` element.
    pub contains_head: bool,
    /// Whether the component propagates head content.
    pub propagation: bool,
}

// Keep the old name as a type alias during migration
/// Deprecated: Use [`TransformResult`] instead.
pub type AstroCodegenReturn = TransformResult;

/// Astro code generator.
///
/// Transforms an `AstroRoot` AST into JavaScript code that can be executed
/// by the Astro runtime.
pub struct AstroCodegen<'a> {
    allocator: &'a Allocator,
    options: TransformOptions,
    /// Output buffer
    code: CodeBuffer,
    /// Source text of the original Astro file
    source_text: &'a str,
    /// Result from the scanner pass (populated in `build()`)
    scan_result: Option<ScanResult>,
    /// Track if we're inside head element
    in_head: bool,
    /// Track if we've already inserted $$maybeRenderHead or $$renderHead
    render_head_inserted: bool,
    /// Track if we've seen an explicit <head> element (which uses $$renderHead)
    has_explicit_head: bool,
    /// Collected module imports for metadata
    module_imports: Vec<ModuleImport>,
    /// Map of component names to their import specifiers (for client:only resolution)
    component_imports: FxHashMap<String, ComponentImportInfo>,
    /// When true, skip printing slot="..." attributes on elements (used for conditional slots)
    skip_slot_attribute: bool,
    /// Current script index for $$renderScript URLs
    script_index: usize,
    /// Current element nesting depth (0 = root level)
    element_depth: usize,
    /// Counter for generating unique transition hashes
    transition_counter: std::cell::Cell<usize>,
    /// Base hash for the source file (computed once)
    source_hash: String,
}

/// Information about an imported module for metadata.
#[derive(Debug, Clone)]
struct ModuleImport {
    specifier: String,
    /// Variable name for the namespace import (e.g., `$$module1`)
    namespace_var: String,
    /// Import assertion string (e.g., `{type:"json"}`)
    assertion: String,
}

/// Information about an imported component.
#[derive(Debug, Clone)]
struct ComponentImportInfo {
    /// The import specifier (e.g., "../components")
    specifier: String,
    /// The export name ("default" for default imports, otherwise the named export)
    export_name: String,
    /// Whether this is a namespace import (import * as x)
    is_namespace: bool,
}

// Internal types (HoistedScript, HydratedComponent, etc.) are now defined in scanner.rs

impl<'a> AstroCodegen<'a> {
    /// Create a new Astro codegen instance.
    pub fn new(allocator: &'a Allocator, source_text: &'a str, options: TransformOptions) -> Self {
        // Compute base hash for the source file (similar to Go compiler's HashString)
        let source_hash = Self::compute_source_hash(source_text);

        Self {
            allocator,
            options,
            code: CodeBuffer::default(),
            source_text,
            scan_result: None,
            in_head: false,
            render_head_inserted: false,
            has_explicit_head: false,
            module_imports: Vec::new(),
            component_imports: FxHashMap::default(),
            skip_slot_attribute: false,
            script_index: 0,
            element_depth: 0,
            transition_counter: std::cell::Cell::new(0),
            source_hash,
        }
    }

    /// Compute a hash of the source text (similar to Go's xxhash + base32)
    fn compute_source_hash(source: &str) -> String {
        use std::collections::hash_map::DefaultHasher;
        use std::hash::{Hash, Hasher};

        let mut hasher = DefaultHasher::new();
        source.hash(&mut hasher);
        let hash = hasher.finish();
        // Convert to base32-like lowercase alphanumeric (8 chars)
        Self::to_base32_like(hash)
    }

    /// Convert a u64 hash to a lowercase alphanumeric string (similar to base32)
    fn to_base32_like(hash: u64) -> String {
        const ALPHABET: &[u8] = b"abcdefghijklmnopqrstuvwxyz234567";
        let mut result = String::with_capacity(8);
        let mut h = hash;
        for _ in 0..8 {
            let idx = (h & 0x1f) as usize;
            result.push(ALPHABET[idx] as char);
            h >>= 5;
        }
        result
    }

    // --- Scan result accessors ---

    fn scan(&self) -> &ScanResult {
        self.scan_result.as_ref().expect("scanner must run before printing")
    }

    fn uses_astro_global(&self) -> bool {
        self.scan().uses_astro_global
    }

    fn uses_transitions(&self) -> bool {
        self.scan().uses_transitions
    }

    fn is_client_only_component(&self, name: &str) -> bool {
        self.scan().client_only_component_names.contains(name)
    }

    fn hoisted_scripts(&self) -> &[HoistedScript] {
        &self.scan().hoisted_scripts
    }

    fn hydrated_components(&self) -> &[HydratedComponent] {
        &self.scan().hydrated_components
    }

    fn hydration_directives(&self) -> &[String] {
        &self.scan().hydration_directives
    }

    fn scanned_client_only_components(&self) -> &[HydratedComponent] {
        &self.scan().client_only_components
    }

    fn server_deferred_components(&self) -> &[HydratedComponent] {
        &self.scan().server_deferred_components
    }

    /// Resolve a component name (possibly dot-notation like `"Two.someName"`) to
    /// its metadata, looking up the root import name in `component_imports`.
    ///
    /// This mirrors the Go compiler's `ExtractComponentExportName` logic:
    /// - `import One from '...'` + `<One />` → export_name = `"default"`
    /// - `import { Three } from '...'` + `<Three />` → export_name = `"Three"`
    /// - `import * as Two from '...'` + `<Two.someName />` → export_name = `"someName"`
    /// - `import * as four from '...'` + `<four.nested.deep.Component />` → export_name = `"nested.deep.Component"`
    /// - `import Foo from '...'` + `<Foo.Bar />` → export_name = `"default.Bar"`
    fn resolve_component_metadata(&self, component_name: &str) -> Option<TransformResultHydratedComponent> {
        if component_name.contains('.') {
            // Dot-notation: split into root and rest
            let dot_pos = component_name.find('.').unwrap();
            let root = &component_name[..dot_pos];
            let rest = &component_name[dot_pos + 1..];

            let info = self.component_imports.get(root)?;
            let export_name = if info.is_namespace {
                // import * as Root from '...' → export_name is the property path
                rest.to_string()
            } else if info.export_name == "default" {
                // import Root from '...' → export_name is "default.Rest"
                format!("default.{rest}")
            } else {
                // import { Root } from '...' → export_name is the full component name
                component_name.to_string()
            };

            let resolved_path = self.options.resolve_specifier(&info.specifier);
            Some(TransformResultHydratedComponent {
                export_name,
                local_name: component_name.to_string(),
                specifier: info.specifier.clone(),
                resolved_path,
            })
        } else {
            // Simple name: direct lookup
            let info = self.component_imports.get(component_name)?;
            let resolved_path = self.options.resolve_specifier(&info.specifier);
            Some(TransformResultHydratedComponent {
                export_name: info.export_name.clone(),
                local_name: component_name.to_string(),
                specifier: info.specifier.clone(),
                resolved_path,
            })
        }
    }

    /// Check if the source contains `await` and thus needs async functions.
    /// Detected by the scanner via AST visitor (no false positives from strings/comments).
    fn needs_async(&self) -> bool {
        self.scan_result.as_ref().is_some_and(|r| r.has_await)
    }

    /// Get the async function prefix if needed ("async " or "").
    fn get_async_prefix(&self) -> &'static str {
        if self.needs_async() { "async " } else { "" }
    }

    /// Get the slot callback parameter list.
    ///
    /// When `resultScopedSlot` is enabled, slots receive the `$$result` render context
    /// parameter: `($$result) => ...`. Otherwise, they use an empty parameter list: `() => ...`.
    fn get_slot_params(&self) -> &'static str {
        if self.options.result_scoped_slot { "($$result) => " } else { "() => " }
    }

    /// Build the JavaScript output from an Astro AST.
    pub fn build(mut self, root: &'a AstroRoot<'a>) -> TransformResult {
        // Phase 1: Scan — collect metadata in a single AST walk
        let scan_result = AstroScanner::new(self.allocator).scan(root);
        self.scan_result = Some(scan_result);

        // Phase 2: Print — emit code using the collected metadata (may contain TS syntax)
        self.print_astro_root(root);

        // Build public hoisted scripts from internal representation
        let scripts = self
            .hoisted_scripts()
            .iter()
            .map(|s| match s.script_type {
                InternalHoistedScriptType::External => TransformResultHoistedScript {
                    script_type: HoistedScriptType::External,
                    src: s.src.clone(),
                    code: None,
                },
                InternalHoistedScriptType::Inline | InternalHoistedScriptType::DefineVars => {
                    TransformResultHoistedScript {
                        script_type: HoistedScriptType::Inline,
                        code: s.value.clone(),
                        src: None,
                    }
                }
            })
            .collect();

        // Build public hydrated components from internal representation
        let hydrated_components = self
            .hydrated_components()
            .iter()
            .filter_map(|h| {
                self.resolve_component_metadata(&h.name)
            })
            .collect();

        // Build public client-only components from scanner's full names
        let client_only_components = self
            .scanned_client_only_components()
            .iter()
            .filter_map(|h| {
                self.resolve_component_metadata(&h.name)
            })
            .collect();

        // Build public server components from internal representation
        let server_components = self
            .server_deferred_components()
            .iter()
            .filter_map(|h| {
                let mut meta = self.resolve_component_metadata(&h.name)?;
                // Server components set local_name to the tag name from the template
                // (matching Go compiler behavior)
                meta.local_name.clone_from(&h.name);
                Some(meta)
            })
            .collect();

        let propagation = self.uses_transitions();
        let contains_head = self.has_explicit_head;
        let scope = self.source_hash.clone();
        let code = self.code.into_string();

        // Phase 3: Strip TypeScript from the generated code.
        // The codegen output is valid TypeScript (frontmatter + template expressions
        // may contain `as` casts, interfaces, type annotations, etc.). We parse it
        // as TS, run the transformer, and re-emit as JavaScript.
        let code = strip_typescript(self.allocator, &code);

        TransformResult {
            code,
            map: String::new(),
            scope,
            style_error: Vec::new(),
            diagnostics: Vec::new(),
            css: Vec::new(),
            scripts,
            hydrated_components,
            client_only_components,
            server_components,
            contains_head,
            propagation,
        }
    }

    fn print(&mut self, s: &str) {
        self.code.print_str(s);
    }

    fn println(&mut self, s: &str) {
        self.code.print_str(s);
        self.code.print_char('\n');
    }

    fn print_astro_root(&mut self, root: &'a AstroRoot<'a>) {
        // 1. Print internal imports
        self.print_internal_imports();

        // 2. Extract and print user imports from frontmatter
        let (imports, exports, other_statements) =
            self.split_frontmatter(root.frontmatter.as_deref());

        // Print user imports
        for import in &imports {
            self.print_statement(import);
        }

        // Blank line after user imports (matches Go compiler)
        if !imports.is_empty() {
            self.println("");
        }

        // 3. Print namespace imports for modules (for metadata) - skip client:only components
        self.print_namespace_imports();

        // 4. Print metadata export
        // Blank line before metadata when there are no user imports
        if imports.is_empty() {
            self.println("");
        }
        self.print_metadata();

        // 5. Print top-level Astro global if needed (before exports, per Go compiler)
        if self.uses_astro_global() {
            self.print_top_level_astro();
        }

        // 6. Print hoisted exports (after $$Astro, before component)
        for export_stmt in &exports {
            self.print_statement(export_stmt);
        }

        // 7. Print the component
        let component_name = get_component_name(self.options.filename.as_deref());
        self.print_component_wrapper(&other_statements, &root.body, &component_name);

        // 8. Print default export
        self.println(&format!("export default {component_name};"));
    }

    // === Printing methods below — all analysis is done by the scanner ===

    fn print_internal_imports(&mut self) {
        let url = self.options.get_internal_url().to_string();

        self.println("import {");
        self.println(&format!("  {},", runtime::FRAGMENT));
        self.println(&format!("  render as {},", runtime::RENDER));
        self.println(&format!("  createAstro as {},", runtime::CREATE_ASTRO));
        self.println(&format!("  createComponent as {},", runtime::CREATE_COMPONENT));
        self.println(&format!("  renderComponent as {},", runtime::RENDER_COMPONENT));
        self.println(&format!("  renderHead as {},", runtime::RENDER_HEAD));
        self.println(&format!("  maybeRenderHead as {},", runtime::MAYBE_RENDER_HEAD));
        self.println(&format!("  unescapeHTML as {},", runtime::UNESCAPE_HTML));
        self.println(&format!("  renderSlot as {},", runtime::RENDER_SLOT));
        self.println(&format!("  mergeSlots as {},", runtime::MERGE_SLOTS));
        self.println(&format!("  addAttribute as {},", runtime::ADD_ATTRIBUTE));
        self.println(&format!("  spreadAttributes as {},", runtime::SPREAD_ATTRIBUTES));
        self.println(&format!("  defineStyleVars as {},", runtime::DEFINE_STYLE_VARS));
        self.println(&format!("  defineScriptVars as {},", runtime::DEFINE_SCRIPT_VARS));
        self.println(&format!("  renderTransition as {},", runtime::RENDER_TRANSITION));
        self.println(&format!("  createTransitionScope as {},", runtime::CREATE_TRANSITION_SCOPE));
        self.println(&format!("  renderScript as {},", runtime::RENDER_SCRIPT));
        // Only import $$createMetadata when no custom resolvePath is provided
        // (needed for runtime $$metadata.resolvePath() fallback)
        if !self.options.has_resolve_path() {
            self.println(&format!("  createMetadata as {}", runtime::CREATE_METADATA));
        }
        self.println(&format!("}} from \"{url}\";"));

        // Add transitions.css import if transitions are used
        if self.uses_transitions() {
            self.println("import \"transitions.css\";");
        }
    }

    fn print_namespace_imports(&mut self) {
        // Skip $$module* re-imports when resolvePath is provided
        // (they're only needed for $$createMetadata runtime fallback)
        if self.module_imports.is_empty() || self.options.has_resolve_path() {
            return;
        }

        // Clone the imports to avoid borrow issues
        let imports: Vec<_> = self.module_imports.clone();
        for import in imports {
            // Include assertion/with clause if present
            if import.assertion == "{}" {
                self.println(&format!(
                    "import * as {} from \"{}\";",
                    import.namespace_var, import.specifier
                ));
            } else {
                self.println(&format!(
                    "import * as {} from \"{}\" assert {};",
                    import.namespace_var, import.specifier, import.assertion
                ));
            }
        }
        self.println("");
    }

    fn print_metadata(&mut self) {
        // Skip entire $$metadata emission when resolvePath is provided
        // (matching Go compiler: opts.ResolvePath != nil → early return)
        if self.options.has_resolve_path() {
            return;
        }

        // Build modules array
        let modules_str = if self.module_imports.is_empty() {
            "[]".to_string()
        } else {
            let items: Vec<String> = self
                .module_imports
                .iter()
                .map(|m| {
                    format!(
                        "{{ module: {}, specifier: \"{}\", assert: {} }}",
                        m.namespace_var, m.specifier, m.assertion
                    )
                })
                .collect();
            format!("[{}]", items.join(", "))
        };

        // Build hydrated components array
        // Go compiler outputs: custom elements first (quoted), then regular components in reverse order
        let hydrated_str = if self.hydrated_components().is_empty() {
            "[]".to_string()
        } else {
            // Separate custom elements and regular components
            let custom_elements: Vec<String> = self
                .hydrated_components()
                .iter()
                .filter(|c| c.is_custom_element)
                .map(|c| format!("\"{}\"", c.name))
                .collect();

            // Regular components in reverse order
            let regular_components: Vec<String> = self
                .hydrated_components()
                .iter()
                .filter(|c| !c.is_custom_element)
                .rev()
                .map(|c| c.name.clone())
                .collect();

            // Combine: custom elements first, then reversed regular components
            let mut items = custom_elements;
            items.extend(regular_components);
            format!("[{}]", items.join(","))
        };

        // Build client-only components array for $$metadata
        // The Go compiler uses import specifiers here, not component names
        let client_only_str = {
            let scan_client_only = self.scanned_client_only_components();
            if scan_client_only.is_empty() {
                "[]".to_string()
            } else {
                let mut seen = Vec::new();
                let mut items = Vec::new();
                for h in scan_client_only {
                    // For dot-notation, look up the root part
                    let root = if h.name.contains('.') {
                        &h.name[..h.name.find('.').unwrap()]
                    } else {
                        h.name.as_str()
                    };
                    if let Some(info) = self.component_imports.get(root)
                        && !seen.contains(&info.specifier)
                    {
                        items.push(format!("\"{}\"", info.specifier));
                        seen.push(info.specifier.clone());
                    }
                }
                format!("[{}]", items.join(", "))
            }
        };

        // Build hydration directives set
        let directives_str = if self.hydration_directives().is_empty() {
            "new Set([])".to_string()
        } else {
            let items: Vec<String> =
                self.hydration_directives().iter().map(|s| format!("\"{s}\"")).collect();
            format!("new Set([{}])", items.join(", "))
        };

        // Build hoisted scripts array
        let hoisted_str = if self.hoisted_scripts().is_empty() {
            "[]".to_string()
        } else {
            let items: Vec<String> = self
                .hoisted_scripts()
                .iter()
                .map(|script| {
                    match script.script_type {
                        InternalHoistedScriptType::Inline => {
                            let value = script.value.as_deref().unwrap_or("");
                            // Escape backticks and interpolation for template literal
                            let escaped = escape_template_literal(value);
                            format!("{{ type: \"inline\", value: `{escaped}` }}")
                        }
                        InternalHoistedScriptType::External => {
                            let src = script.src.as_deref().unwrap_or("");
                            // Escape single quotes for string literal
                            let escaped = escape_single_quote(src);
                            format!("{{ type: \"external\", src: '{escaped}' }}")
                        }
                        InternalHoistedScriptType::DefineVars => {
                            let value = script.value.as_deref().unwrap_or("");
                            let keys = script.keys.as_deref().unwrap_or("");
                            let escaped_value = escape_template_literal(value);
                            let escaped_keys = escape_single_quote(keys);
                            format!(
                                "{{ type: \"define:vars\", value: `{escaped_value}`, keys: '{escaped_keys}' }}"
                            )
                        }
                    }
                })
                .collect();
            format!("[{}]", items.join(", "))
        };

        // Use filename if provided, otherwise import.meta.url
        let metadata_url = match &self.options.filename {
            Some(f) => format!("\"{}\"", escape_single_quote(f)),
            None => "import.meta.url".to_string(),
        };

        self.println(&format!(
            "export const $$metadata = {}({}, {{ modules: {}, hydratedComponents: {}, clientOnlyComponents: {}, hydrationDirectives: {}, hoisted: {} }});",
            runtime::CREATE_METADATA,
            metadata_url,
            modules_str,
            hydrated_str,
            client_only_str,
            directives_str,
            hoisted_str
        ));
        self.println("");
    }

    fn print_top_level_astro(&mut self) {
        // Get the Astro global args from options, default to standard URL
        let astro_global_args =
            self.options.astro_global_args.as_deref().unwrap_or("\"https://astro.build\"");

        self.println(&format!("const $$Astro = {}({});", runtime::CREATE_ASTRO, astro_global_args));
        self.println("const Astro = $$Astro;");
    }

    /// Split frontmatter into three categories:
    /// - imports: import declarations (hoisted to top of module)
    /// - exports: export declarations (hoisted after metadata, before component)
    /// - other: regular statements (inside component function)
    fn split_frontmatter<'b>(
        &mut self,
        frontmatter: Option<&'b AstroFrontmatter<'a>>,
    ) -> (Vec<&'b Statement<'a>>, Vec<&'b Statement<'a>>, Vec<&'b Statement<'a>>)
    where
        'a: 'b,
    {
        let mut imports = Vec::new();
        let mut exports = Vec::new();
        let mut other = Vec::new();

        if let Some(fm) = frontmatter {
            let mut module_counter = 1;

            for stmt in &fm.program.body {
                // Check for export statements first - they need to be hoisted
                if matches!(
                    stmt,
                    Statement::ExportNamedDeclaration(_)
                        | Statement::ExportDefaultDeclaration(_)
                        | Statement::ExportAllDeclaration(_)
                ) {
                    exports.push(stmt);
                    continue;
                }

                if let Statement::ImportDeclaration(import) = stmt {
                    // Track module import for metadata
                    let source = import.source.value.as_str();

                    // Extract component names from the import specifiers
                    if import.import_kind != ImportOrExportKind::Type {
                        if let Some(specifiers) = &import.specifiers {
                            for spec in specifiers {
                                match spec {
                                    ImportDeclarationSpecifier::ImportDefaultSpecifier(
                                        default_spec,
                                    ) => {
                                        let local_name =
                                            default_spec.local.name.as_str().to_string();
                                        self.component_imports.insert(
                                            local_name,
                                            ComponentImportInfo {
                                                specifier: source.to_string(),
                                                export_name: "default".to_string(),
                                                is_namespace: false,
                                            },
                                        );
                                    }
                                    ImportDeclarationSpecifier::ImportSpecifier(named_spec) => {
                                        let local_name = named_spec.local.name.as_str().to_string();
                                        let imported_name =
                                            named_spec.imported.name().as_str().to_string();
                                        self.component_imports.insert(
                                            local_name,
                                            ComponentImportInfo {
                                                specifier: source.to_string(),
                                                export_name: imported_name,
                                                is_namespace: false,
                                            },
                                        );
                                    }
                                    ImportDeclarationSpecifier::ImportNamespaceSpecifier(
                                        ns_spec,
                                    ) => {
                                        let local_name = ns_spec.local.name.as_str().to_string();
                                        self.component_imports.insert(
                                            local_name,
                                            ComponentImportInfo {
                                                specifier: source.to_string(),
                                                export_name: "*".to_string(),
                                                is_namespace: true,
                                            },
                                        );
                                    }
                                }
                            }
                        }

                        // Check if any of the imported names are client:only components
                        let is_client_only_import = if let Some(specifiers) = &import.specifiers {
                            specifiers.iter().any(|spec| {
                                let local_name = match spec {
                                    ImportDeclarationSpecifier::ImportDefaultSpecifier(s) => {
                                        s.local.name.as_str()
                                    }
                                    ImportDeclarationSpecifier::ImportSpecifier(s) => {
                                        s.local.name.as_str()
                                    }
                                    ImportDeclarationSpecifier::ImportNamespaceSpecifier(s) => {
                                        s.local.name.as_str()
                                    }
                                };
                                self.is_client_only_component(local_name)
                            })
                        } else {
                            false
                        };

                        // Skip bare CSS imports from $$module re-imports
                        let is_bare_css_import = import.specifiers.is_none()
                            && is_css_specifier(source);

                        if is_client_only_import {
                            // Client:only component imports are not needed at runtime
                            // on the server — the component reference is `null` in
                            // renderComponent(). Don't emit the import at all so we
                            // don't pull client-side framework code into the SSR bundle.
                            continue;
                        } else if is_bare_css_import {
                            // Bare CSS imports don't need metadata tracking, but the
                            // import itself should still be emitted.
                            imports.push(stmt);
                        } else {
                            // Normal import - emit and add to modules
                            imports.push(stmt);
                            let namespace_var = format!("$$module{module_counter}");

                            // Extract import assertion/with clause if present
                            let assertion = if let Some(with_clause) = &import.with_clause {
                                let items: Vec<String> = with_clause
                                    .with_entries
                                    .iter()
                                    .map(|attr| {
                                        let key = match &attr.key {
                                            oxc_ast::ast::ImportAttributeKey::Identifier(id) => {
                                                id.name.as_str().to_string()
                                            }
                                            oxc_ast::ast::ImportAttributeKey::StringLiteral(
                                                lit,
                                            ) => format!("\"{}\"", lit.value.as_str()),
                                        };
                                        format!("{}:\"{}\"", key, attr.value.value.as_str())
                                    })
                                    .collect();
                                format!("{{{}}}", items.join(","))
                            } else {
                                "{}".to_string()
                            };

                            self.module_imports.push(ModuleImport {
                                specifier: source.to_string(),
                                namespace_var,
                                assertion,
                            });
                            module_counter += 1;
                        }
                    } else {
                        // Type-only imports (`import type { ... }`) are emitted so
                        // that `strip_typescript` can see and remove them.
                        imports.push(stmt);
                    }
                } else {
                    other.push(stmt);
                }
            }
        }

        (imports, exports, other)
    }

    fn print_statement(&mut self, stmt: &Statement<'_>) {
        // Use the regular codegen for statements
        let mut codegen = Codegen::new();
        stmt.print(&mut codegen, Context::default().with_typescript());
        let code = codegen.into_source_text();
        // Trim trailing newline if present (Codegen may add one)
        let code = code.trim_end_matches('\n');
        self.println(code);
    }

    fn print_component_wrapper(
        &mut self,
        statements: &[&'a Statement<'a>],
        body: &[JSXChild<'a>],
        component_name: &str,
    ) {
        // Use async if source contains await
        let async_prefix = self.get_async_prefix();
        self.println(&format!(
            "const {} = {}({}({}, $$props, $$slots) => {{",
            component_name,
            runtime::CREATE_COMPONENT,
            async_prefix,
            runtime::RESULT
        ));

        // Print Astro setup inside component if needed
        if self.uses_astro_global() {
            self.println(&format!(
                "const Astro = {}.createAstro($$props, $$slots);",
                runtime::RESULT
            ));
            self.println(&format!("Astro.self = {component_name};"));
        }

        // Empty line at start of component body (matches Go compiler)
        self.println("");

        // Print non-import frontmatter statements
        for stmt in statements {
            self.print_statement(stmt);
        }

        // Empty line after frontmatter statements, before return (matches Go compiler)
        if !statements.is_empty() {
            self.println("");
        }

        // Generate the return statement with the template
        self.print("return ");
        self.print(runtime::RENDER);
        self.print("`");

        // Check if we need to insert $$maybeRenderHead at the start of the template
        // This is needed when there's no explicit <head> element and we have body content
        if self.needs_maybe_render_head_at_start(body) {
            self.print(&format!("${{{}({})}}", runtime::MAYBE_RENDER_HEAD, runtime::RESULT));
            self.render_head_inserted = true;
        }

        // Convert JSX body to template literal content
        // Skip leading whitespace-only text nodes (Go compiler strips these)
        self.print_jsx_children_skip_leading_whitespace(body);

        self.println("`;");

        // Filename: use single-quoted string (to match Go compiler), or undefined
        // Go compiler uses single quotes here, escaping double quotes inside
        let filename_part = match &self.options.filename {
            Some(f) => format!("'{}'", escape_single_quote(f)),
            None => "undefined".to_string(),
        };
        // Third argument: "self" when transitions are used, undefined otherwise
        let propagation = if self.uses_transitions() { "\"self\"" } else { "undefined" };
        self.println(&format!("}}, {filename_part}, {propagation});"));
    }

    /// Print JSX children, skipping leading whitespace-only text nodes.
    /// This matches the Go compiler behavior which strips leading whitespace
    /// after the template literal opening backtick.
    fn print_jsx_children_skip_leading_whitespace(&mut self, children: &[JSXChild<'a>]) {
        let mut started = false;
        for child in children {
            if !started {
                // Skip leading whitespace-only text nodes
                if let JSXChild::Text(text) = child
                    && text.value.trim().is_empty()
                {
                    continue;
                }
                // AstroDoctype is always stripped (prints nothing), so don't
                // consider it the start of real content. This ensures
                // whitespace after a doctype is still skipped.
                if matches!(child, JSXChild::AstroDoctype(_)) {
                    continue;
                }
                started = true;
            }
            self.print_jsx_child(child);
        }
    }

    /// Check if we need to insert $$maybeRenderHead at the start of the template.
    /// This is true when:
    /// - There's no explicit <html><head> structure
    /// - The first non-whitespace element is a body element or body content element
    fn needs_maybe_render_head_at_start(&self, body: &[JSXChild<'a>]) -> bool {
        // If we've already handled render head (e.g., from an explicit <head>), no need
        if self.render_head_inserted || self.has_explicit_head {
            return false;
        }

        // Find the first significant element
        for child in body {
            match child {
                JSXChild::Text(text) => {
                    // Skip whitespace-only text
                    if text.value.trim().is_empty() {
                        continue;
                    }
                    // Non-whitespace text - no render head needed for text
                    return false;
                }
                JSXChild::Element(el) => {
                    let name = get_jsx_element_name(&el.opening_element.name);
                    // If we find html, it will contain head/body and we don't need it here
                    if name == "html" {
                        return false;
                    }
                    // <slot> is rendered as $$renderSlot(), not HTML - doesn't need maybeRenderHead
                    if name == "slot" {
                        return false;
                    }
                    // Skip <script> elements with AstroScript children (they're hoisted, not rendered)
                    if name == "script"
                        && el.children.iter().any(|c| matches!(c, JSXChild::AstroScript(_)))
                    {
                        continue;
                    }
                    // For body or any non-head HTML element (not a component or custom element), we need maybeRenderHead
                    return !is_component_name(&name)
                        && !is_custom_element(&name)
                        && !is_head_element(&name);
                }
                JSXChild::Fragment(_) | JSXChild::ExpressionContainer(_) => {
                    // Fragment/expression doesn't need render head
                    return false;
                }
                _ => {}
            }
        }
        false
    }

    /// Check if this element needs `$$maybeRenderHead` inserted before it.
    /// Returns true for non-component HTML elements (body content).
    fn needs_render_head(&self, name: &str) -> bool {
        // Don't insert if already done, or if we're in the head
        if self.render_head_inserted || self.in_head {
            return false;
        }
        // Components (uppercase or contains dot) don't trigger maybeRenderHead
        if is_component_name(name) {
            return false;
        }
        // Custom elements (contain dash) are treated as components
        if is_custom_element(name) {
            return false;
        }
        // Don't insert for head-related elements that can appear in <head>
        // This matches the Go compiler's skip list in print-to-js.go lines 416-418
        if is_head_element(name) {
            return false;
        }
        // Don't insert before body if we had an explicit <head>
        // ($$renderHead was already called inside head)
        if name == "body" && self.has_explicit_head {
            return false;
        }
        // For body without explicit head, or for other HTML elements: insert maybeRenderHead
        true
    }

    /// Insert `$$maybeRenderHead` if needed before an HTML element.
    fn maybe_insert_render_head(&mut self, name: &str) {
        if self.needs_render_head(name) {
            self.print(&format!("${{{}({})}}", runtime::MAYBE_RENDER_HEAD, runtime::RESULT));
            self.render_head_inserted = true;
        }
    }

    fn print_jsx_child(&mut self, child: &JSXChild<'a>) {
        match child {
            JSXChild::Text(text) => {
                self.print_jsx_text(text);
            }
            JSXChild::Element(el) => {
                self.print_jsx_element(el);
            }
            JSXChild::Fragment(frag) => {
                self.print_jsx_fragment(frag);
            }
            JSXChild::ExpressionContainer(expr) => {
                self.print_jsx_expression_container(expr);
            }
            JSXChild::Spread(spread) => {
                self.print_jsx_spread_child(spread);
            }
            JSXChild::AstroScript(_script) => {
                // AstroScript is handled specially - it's already parsed TypeScript
                // For now, we skip it in the template output
                // TODO: Handle inline scripts
            }
            JSXChild::AstroDoctype(_doctype) => {
                // Doctype is typically stripped in the output
                // The runtime handles doctype separately
            }
            JSXChild::AstroComment(comment) => {
                self.print_astro_comment(comment);
            }
        }
    }

    fn print_astro_comment(&mut self, comment: &AstroComment<'a>) {
        // Emit the HTML comment with escaped backticks (for template literal safety)
        self.print("<!--");
        self.print(&escape_template_literal(comment.value.as_str()));
        self.print("-->");
    }

    fn print_jsx_text(&mut self, text: &JSXText<'a>) {
        // Escape backticks and ${} in text content
        let escaped = escape_template_literal(text.value.as_str());
        self.print(&escaped);
    }

    fn print_jsx_element(&mut self, el: &JSXElement<'a>) {
        let name = get_jsx_element_name(&el.opening_element.name);

        // Handle <script> elements that should be hoisted (have AstroScript children or `hoist` attribute)
        if name == "script" && Self::is_hoisted_script(el) {
            // At root level (depth 0), hoisted scripts are completely removed from template
            // When nested inside another element (depth > 0), emit $$renderScript call
            if self.element_depth > 0 {
                let filename = self
                    .options
                    .filename
                    .clone()
                    .unwrap_or_else(|| "/src/pages/index.astro".to_string());
                let index = self.script_index;
                self.script_index += 1;

                self.print("${");
                self.print(runtime::RENDER_SCRIPT);
                self.print("(");
                self.print(runtime::RESULT);
                self.print(",\"");
                self.print(&filename);
                self.print("?astro&type=script&index=");
                self.print(&index.to_string());
                self.print("&lang.ts\")}");
            }
            // At root level, skip the script entirely (it's hoisted to metadata)
            return;
        }

        // Check if this is a component (starts with uppercase or contains dot)
        // or a custom element (contains dash, like web components)
        let is_component = is_component_name(&name);
        let is_custom = is_custom_element(&name);

        // Increment depth before processing element children
        self.element_depth += 1;

        if is_component || is_custom {
            self.print_component_element(el, &name);
        } else {
            self.print_html_element(el, &name);
        }

        // Decrement depth after processing
        self.element_depth -= 1;
    }

    /// Check if a script element should be hoisted.
    /// A script is hoisted if it meets the attribute criteria AND has content.
    fn is_hoisted_script(el: &JSXElement<'a>) -> bool {
        // First check if attributes indicate hoisting
        if !should_hoist_script(&el.opening_element.attributes) {
            return false;
        }

        // Check if it has AstroScript child (parsed content)
        if el.children.iter().any(|child| matches!(child, JSXChild::AstroScript(_))) {
            return true;
        }

        // Check if it has non-empty text content
        let has_text_content = el.children.iter().any(|child| {
            if let JSXChild::Text(text) = child { !text.value.trim().is_empty() } else { false }
        });

        // Check for src attribute (external scripts are hoisted even without content)
        let has_src = el.opening_element.attributes.iter().any(|attr| {
            if let JSXAttributeItem::Attribute(attr) = attr {
                get_jsx_attribute_name(&attr.name) == "src"
            } else {
                false
            }
        });

        has_text_content || has_src
    }

    fn print_component_element(&mut self, el: &JSXElement<'a>, name: &str) {
        // Check for client:* directives
        let mut hydration_info = Self::extract_hydration_info(&el.opening_element.attributes);

        // Check if this is a custom element (has dash in name)
        let is_custom = is_custom_element(name);

        // For ALL hydrated components (not just client:only), resolve component path and export
        // This info is used for client:component-path and client:component-export attributes
        if hydration_info.directive.is_some() {
            // Handle member expressions like "components.A" or "defaultImport.Counter1"
            if name.contains('.') {
                let parts: Vec<&str> = name.split('.').collect();
                let namespace = parts[0];
                let property = parts[1..].join(".");

                if let Some(import_info) = self.component_imports.get(namespace) {
                    hydration_info.component_path = Some(import_info.specifier.clone());
                    // For namespace imports (import * as x), the export is just the property name
                    // For default imports (import x from), the export is "default.Property"
                    if import_info.is_namespace {
                        hydration_info.component_export = Some(property);
                    } else {
                        // Default or named import - prepend the original export name
                        hydration_info.component_export =
                            Some(format!("{}.{}", import_info.export_name, property));
                    }
                }
            } else if let Some(import_info) = self.component_imports.get(name) {
                hydration_info.component_path = Some(import_info.specifier.clone());
                hydration_info.component_export = Some(import_info.export_name.clone());
            }

            // Hydrated component tracking is done by the scanner
        }

        // Check for set:html or set:text on components (including Fragment)
        // <Component set:html={...} /> becomes $$renderComponent with a default slot containing $$unescapeHTML
        // <Component set:text={...} /> becomes $$renderComponent with a default slot containing the value directly
        let set_directive = Self::extract_set_html_value(&el.opening_element.attributes);

        self.print("${");
        self.print(runtime::RENDER_COMPONENT);
        self.print("(");
        self.print(runtime::RESULT);
        self.print(",\"");
        self.print(name);
        self.print("\",");

        // Component reference - for client:only it's null, for custom elements it's
        // a quoted string, otherwise it's the component identifier
        if hydration_info.is_client_only {
            self.print("null");
        } else if is_custom {
            // Custom elements use quoted tag name: "my-element"
            self.print("\"");
            self.print(name);
            self.print("\"");
        } else {
            self.print(name);
        }

        self.print(",{");

        // Components always receive slot as a prop (matching Go compiler behavior).
        // Only HTML elements have the slot attribute stripped when inside named slots.
        let prev_skip_slot = self.skip_slot_attribute;
        self.skip_slot_attribute = false;

        // Print attributes as object properties (skip set:html/set:text if present)
        self.print_component_attributes_filtered(
            &el.opening_element.attributes,
            &hydration_info,
            if set_directive.is_some() { Some(&["set:html", "set:text"]) } else { None },
        );

        self.skip_slot_attribute = prev_skip_slot;

        self.print("}");

        // For set:html or set:text, create a default slot with the content
        if let Some((value, is_html, needs_unescape, is_raw_text)) = set_directive {
            let async_prefix = self.get_async_prefix();
            let slot_params = self.get_slot_params();
            self.print(&format!(",{{\"default\": {async_prefix}{slot_params}"));
            self.print(runtime::RENDER);
            self.print("`");
            if is_raw_text {
                // set:text with string literal - inline raw text without ${}
                self.print(&value);
            } else {
                self.print("${");
                if is_html && needs_unescape {
                    // set:html with expression needs $$unescapeHTML
                    self.print(runtime::UNESCAPE_HTML);
                    self.print("(");
                    self.print(&value);
                    self.print(")");
                } else {
                    // set:html with string literal or set:text with expression - just interpolate directly
                    self.print(&value);
                }
                self.print("}");
            }
            self.print("`,}");
        } else if !el.children.is_empty() {
            // Print slots if there are children
            self.print(",");
            // Custom elements use browser's Shadow DOM slots, not Astro slots
            // All children go to default slot with their slot attributes preserved
            if is_custom {
                self.print_component_default_slot_only(&el.children);
            } else {
                self.print_component_slots(&el.children);
            }
        }

        self.print(")}");
    }

    /// Extract set:html value from attributes
    /// Extract set:html or set:text value from component attributes
    /// Returns (value_string, is_html, needs_unescape, is_raw_text)
    /// - is_html is true for set:html, false for set:text
    /// - needs_unescape is true for expressions (need $$unescapeHTML), false for literals
    /// - is_raw_text is true for set:text with string literal (should be inlined without ${})
    fn extract_set_html_value(
        attrs: &[JSXAttributeItem<'a>],
    ) -> Option<(String, bool, bool, bool)> {
        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let name = get_jsx_attribute_name(&attr.name);
                let is_html = name == "set:html";
                let is_text = name == "set:text";
                if is_html || is_text {
                    let (value, needs_unescape, is_raw_text) = match &attr.value {
                        Some(JSXAttributeValue::StringLiteral(lit)) => {
                            // String literals don't need $$unescapeHTML
                            // For set:html, decode HTML entities in the string
                            // For set:text, this is raw text to be inlined
                            let raw_value = lit.value.as_str();
                            if is_html {
                                let decoded = decode_html_entities(raw_value);
                                (
                                    Some(format!("\"{}\"", escape_double_quotes(&decoded))),
                                    false,
                                    false,
                                )
                            } else {
                                // set:text with string literal - return raw value for inline
                                (Some(raw_value.to_string()), false, true)
                            }
                        }
                        Some(JSXAttributeValue::ExpressionContainer(expr)) => {
                            let mut needs_unescape = true;

                            if let Some(e) = expr.expression.as_expression() {
                                // Template literals:
                                // - Static (no expressions): don't need $$unescapeHTML
                                // - Dynamic with only empty quasis (e.g., `${var}`): don't need $$unescapeHTML
                                // - Dynamic with non-empty quasis (e.g., `<${tag}>`): need $$unescapeHTML
                                if let Expression::TemplateLiteral(tl) = e {
                                    // Static template literal - no expressions
                                    if tl.expressions.is_empty() {
                                        needs_unescape = false;
                                        // For set:html with static template literal, decode HTML entities
                                        if is_html && tl.quasis.len() == 1 {
                                            let raw = tl.quasis[0].value.raw.as_str();
                                            let decoded = decode_html_entities(raw);
                                            return Some((
                                                format!("`{decoded}`"),
                                                is_html,
                                                false,
                                                false,
                                            ));
                                        }
                                    } else {
                                        // Dynamic template literal - check if all quasis are empty
                                        let all_quasis_empty = tl
                                            .quasis
                                            .iter()
                                            .all(|q| q.value.raw.as_str().trim().is_empty());
                                        if all_quasis_empty {
                                            needs_unescape = false;
                                        }
                                        // If any quasi has content (like `<` or `>`), needs_unescape stays true
                                    }
                                }
                                let mut codegen = Codegen::new();
                                e.print_expr(
                                    &mut codegen,
                                    oxc_syntax::precedence::Precedence::Lowest,
                                    Context::default().with_typescript(),
                                );
                                return Some((
                                    codegen.into_source_text(),
                                    is_html,
                                    needs_unescape,
                                    false,
                                ));
                            }
                            (None, true, false)
                        }
                        _ => (None, false, false),
                    };
                    return value.map(|v| (v, is_html, needs_unescape, is_raw_text));
                }
            }
        }
        None
    }

    /// Print component attributes, optionally filtering out certain names
    fn print_component_attributes_filtered(
        &mut self,
        attrs: &[JSXAttributeItem<'a>],
        hydration: &HydrationInfo,
        skip_names: Option<&[&str]>,
    ) {
        let mut first = true;

        // Pre-scan for transition attributes (but don't print them yet - they come after regular attrs)
        let mut transition_name: Option<String> = None;
        let mut transition_animate: Option<String> = None;
        let mut transition_persist = false;
        let mut transition_persist_props: Option<String> = None;

        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let name = get_jsx_attribute_name(&attr.name);
                if name == "transition:name" {
                    transition_name = Some(Self::get_attr_value_string(attr));
                } else if name == "transition:animate" {
                    transition_animate = Some(Self::get_attr_value_string(attr));
                } else if name == "transition:persist" {
                    transition_persist = true;
                } else if name == "transition:persist-props" {
                    transition_persist_props = Some(Self::get_attr_value_string(attr));
                }
            }
        }

        // Print regular attributes first
        for attr in attrs {
            match attr {
                JSXAttributeItem::Attribute(attr) => {
                    let name = get_jsx_attribute_name(&attr.name);

                    // Skip slot attribute when skip_slot_attribute is true (for non-Fragment components)
                    // For Fragment, we set skip_slot_attribute = false to include slot as a prop
                    if name == "slot" && self.skip_slot_attribute {
                        continue;
                    }

                    // Skip filtered names
                    if let Some(names) = skip_names
                        && names.contains(&name.as_str())
                    {
                        continue;
                    }

                    // Skip transition directives - already handled above
                    if name.starts_with("transition:") {
                        continue;
                    }

                    // Skip is:raw directive - this is handled by the parser as raw text content
                    if name == "is:raw" {
                        continue;
                    }

                    // Skip server:defer directive - handled in metadata, not rendered as a prop
                    if name == "server:defer" {
                        continue;
                    }

                    if !first {
                        self.print(",");
                    }
                    first = false;

                    self.print("\"");
                    self.print(&name);
                    self.print("\":");

                    match &attr.value {
                        None => {
                            self.print("true");
                        }
                        Some(JSXAttributeValue::StringLiteral(lit)) => {
                            self.print("\"");
                            self.print(&escape_double_quotes(lit.value.as_str()));
                            self.print("\"");
                        }
                        Some(JSXAttributeValue::ExpressionContainer(expr)) => {
                            // Template literals and string literals don't need parens
                            // Other expressions should be wrapped in parens
                            if let Some(e) = expr.expression.as_expression() {
                                match e {
                                    Expression::TemplateLiteral(_)
                                    | Expression::StringLiteral(_) => {
                                        self.print_jsx_expression(&expr.expression);
                                    }
                                    _ => {
                                        self.print("(");
                                        self.print_jsx_expression(&expr.expression);
                                        self.print(")");
                                    }
                                }
                            } else {
                                // Empty expression or non-expression
                                self.print("(");
                                self.print_jsx_expression(&expr.expression);
                                self.print(")");
                            }
                        }
                        Some(JSXAttributeValue::Element(_el)) => {
                            // Rare case - JSX element as attribute
                            self.print("\"[JSX]\""); // Placeholder
                        }
                        Some(JSXAttributeValue::Fragment(_)) => {
                            self.print("\"[Fragment]\""); // Placeholder
                        }
                    }
                }
                JSXAttributeItem::SpreadAttribute(spread) => {
                    if !first {
                        self.print(",");
                    }
                    first = false;
                    self.print("...(");
                    self.print_expression(&spread.argument);
                    self.print(")");
                }
            }
        }

        // Print transition attributes AFTER regular attributes
        if transition_name.is_some() || transition_animate.is_some() {
            if !first {
                self.print(",");
            }
            first = false;
            let name_val = transition_name.unwrap_or_else(|| "\"\"".to_string());
            let animate_val = transition_animate.unwrap_or_else(|| "\"\"".to_string());
            let hash = self.generate_transition_hash();
            self.print(&format!(
                "\"data-astro-transition-scope\":({}({}, \"{}\", {}, {}))",
                runtime::RENDER_TRANSITION,
                runtime::RESULT,
                hash,
                animate_val,
                name_val
            ));
        }

        // Print transition:persist-props as a data attribute if present
        if let Some(props_val) = &transition_persist_props {
            if !first {
                self.print(",");
            }
            first = false;
            self.print(&format!("\"data-astro-transition-persist-props\":{props_val}"));
        }

        if transition_persist {
            if !first {
                self.print(",");
            }
            first = false;
            let hash = self.generate_transition_hash();
            self.print(&format!(
                "\"data-astro-transition-persist\":({}({}, \"{}\"))",
                runtime::CREATE_TRANSITION_SCOPE,
                runtime::RESULT,
                hash
            ));
        }

        // Add hydration attributes if present
        if let Some(directive) = &hydration.directive {
            if !first {
                self.print(",");
            }
            self.print(&format!("\"client:component-hydration\":\"{directive}\""));

            // Add path and export for hydrated components (not custom elements)
            // When resolvePath is provided: use plain resolved strings
            // When resolvePath is NOT provided: client:only uses $$metadata.resolvePath() wrapper
            if let Some(path) = &hydration.component_path {
                if hydration.is_client_only && !self.options.has_resolve_path() {
                    // client:only without resolvePath: use runtime $$metadata.resolvePath
                    self.print(&format!(
                        ",\"client:component-path\":($$metadata.resolvePath(\"{path}\"))"
                    ));
                } else {
                    // Regular hydration, or client:only with resolvePath: use plain string with parens
                    self.print(&format!(",\"client:component-path\":(\"{path}\")"));
                }
            }

            if let Some(export) = &hydration.component_export {
                if hydration.is_client_only {
                    // client:only uses plain string without parens
                    self.print(&format!(",\"client:component-export\":\"{export}\""));
                } else {
                    // Regular hydration uses string with parens
                    self.print(&format!(",\"client:component-export\":(\"{export}\")"));
                }
            }
        }
    }

    fn print_html_element(&mut self, el: &JSXElement<'a>, name: &str) {
        // Handle <slot> element specially - it's a slot placeholder, not an HTML element
        // Unless it has is:inline, in which case render as raw HTML
        if name == "slot" && !Self::has_is_inline_attribute(&el.opening_element.attributes) {
            self.print_slot_element(el);
            return;
        }
        // Fall through to render as regular HTML element for is:inline slots

        // Check if this is a special element
        let is_head = name == "head";
        let was_in_head = self.in_head;

        if is_head {
            self.in_head = true;
            self.has_explicit_head = true;
        }

        // Insert $$maybeRenderHead before the first body HTML element
        self.maybe_insert_render_head(name);

        // Extract set:html and set:text directives
        let set_directive = Self::extract_set_directive(&el.opening_element.attributes);

        // Opening tag
        self.print("<");
        self.print(name);

        // Attributes (excluding set:html and set:text)
        self.print_html_attributes(&el.opening_element.attributes);

        // Close the opening tag and handle content
        self.print(">");

        // Handle special head insertion
        if is_head {
            // Children
            for child in &el.children {
                self.print_jsx_child(child);
            }
            // Insert renderHead before closing head tag
            self.print(&format!("${{{}({})}}", runtime::RENDER_HEAD, runtime::RESULT));
            // Mark that head rendering is done - prevents $$maybeRenderHead from being inserted later
            // This matches the Go compiler behavior where printedMaybeHead is set when </head> is printed
            self.render_head_inserted = true;
        } else if let Some((directive_type, value, needs_unescape, is_raw_text)) = set_directive {
            // set:html or set:text directive - inject the content
            if is_raw_text {
                // set:text with string literal - inline raw text without ${}
                self.print(&value);
            } else if directive_type == "html" && needs_unescape {
                // Only use $$unescapeHTML for non-literal expressions
                self.print(&format!("${{{}({})}}", runtime::UNESCAPE_HTML, value));
            } else {
                // For literals (string/template) or set:text expression, just interpolate
                self.print(&format!("${{{value}}}"));
            }
        } else {
            // Regular children
            for child in &el.children {
                self.print_jsx_child(child);
            }
        }

        // Closing tag (skip for void elements like <meta>, <input>, <br>, etc.)
        if !is_void_element(name) {
            self.print("</");
            self.print(name);
            self.print(">");
        }

        if is_head {
            self.in_head = was_in_head;
        }
    }

    /// Print a `<slot>` element as `$$renderSlot` call
    /// `<slot />` -> `$$renderSlot($$result, $$slots["default"])`
    /// `<slot name="foo" />` -> `$$renderSlot($$result, $$slots["foo"])`
    /// `<slot><p>fallback</p></slot>` -> `$$renderSlot($$result, $$slots["default"], $$render\`<p>fallback</p>\`)`
    fn print_slot_element(&mut self, el: &JSXElement<'a>) {
        // Extract slot name from attributes (default is "default")
        let slot_name = Self::extract_slot_name(&el.opening_element.attributes);

        self.print("${");
        self.print(runtime::RENDER_SLOT);
        self.print("(");
        self.print(runtime::RESULT);
        self.print(",$$slots[\"");
        self.print(&slot_name);
        self.print("\"]");

        // Add fallback content if there are children
        if !el.children.is_empty() {
            self.print(",");
            self.print(runtime::RENDER);
            self.print("`");
            for child in &el.children {
                self.print_jsx_child(child);
            }
            self.print("`");
        }

        self.print(")}");
    }

    /// Extract the name attribute from a slot element, defaulting to "default"
    fn extract_slot_name(attrs: &[JSXAttributeItem<'a>]) -> String {
        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let attr_name = get_jsx_attribute_name(&attr.name);
                if attr_name == "name" {
                    if let Some(JSXAttributeValue::StringLiteral(lit)) = &attr.value {
                        return lit.value.to_string();
                    }
                    if let Some(JSXAttributeValue::ExpressionContainer(expr)) = &attr.value {
                        // Dynamic slot name - we need to handle this differently
                        // For now, just use the expression
                        let mut codegen = Codegen::new();
                        if let Some(e) = expr.expression.as_expression() {
                            e.print_expr(
                                &mut codegen,
                                oxc_syntax::precedence::Precedence::Lowest,
                                Context::default().with_typescript(),
                            );
                        }
                        // For dynamic names, we return a placeholder that will need special handling
                        return format!("\" + {} + \"", codegen.into_source_text());
                    }
                }
            }
        }
        "default".to_string()
    }

    /// Check if element has is:inline attribute
    fn has_is_inline_attribute(attrs: &[JSXAttributeItem<'a>]) -> bool {
        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let name = get_jsx_attribute_name(&attr.name);
                if name == "is:inline" {
                    return true;
                }
            }
        }
        false
    }

    /// Extract set:html or set:text directive from attributes
    /// Returns (directive_type, value_string, needs_unescape_html)
    /// Extract set:html/set:text directive from HTML element attributes
    /// Returns (directive_type, value, needs_unescape, is_raw_text)
    /// - is_raw_text is true for set:text with string literal (should be inlined without ${})
    fn extract_set_directive(
        attrs: &[JSXAttributeItem<'a>],
    ) -> Option<(&'static str, String, bool, bool)> {
        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let name = get_jsx_attribute_name(&attr.name);
                if name == "set:html" || name == "set:text" {
                    let directive_type = if name == "set:html" { "html" } else { "text" };
                    let (value, needs_unescape, is_raw_text) = match &attr.value {
                        Some(JSXAttributeValue::StringLiteral(lit)) => {
                            if directive_type == "text" {
                                // set:text with string literal: inline raw text without ${}
                                (lit.value.as_str().to_string(), false, true)
                            } else {
                                // set:html with string literal: use ${"content"}
                                (
                                    format!("\"{}\"", escape_double_quotes(lit.value.as_str())),
                                    false,
                                    false,
                                )
                            }
                        }
                        Some(JSXAttributeValue::ExpressionContainer(expr)) => {
                            let mut codegen = Codegen::new();
                            let mut is_literal = false;
                            if let Some(e) = expr.expression.as_expression() {
                                // Check if expression is a string/template literal
                                is_literal = matches!(
                                    e,
                                    Expression::StringLiteral(_) | Expression::TemplateLiteral(_)
                                );
                                e.print_expr(
                                    &mut codegen,
                                    oxc_syntax::precedence::Precedence::Lowest,
                                    Context::default().with_typescript(),
                                );
                            }
                            // For set:html with non-literal expressions, use $$unescapeHTML
                            // For literals or set:text, don't use $$unescapeHTML
                            let needs_unescape = directive_type == "html" && !is_literal;
                            (codegen.into_source_text(), needs_unescape, false)
                        }
                        _ => ("void 0".to_string(), false, false),
                    };
                    return Some((directive_type, value, needs_unescape, is_raw_text));
                }
            }
        }
        None
    }

    fn print_html_attributes(&mut self, attrs: &[JSXAttributeItem<'a>]) {
        // Check for class + class:list combination that needs merging
        let mut static_class: Option<&str> = None;
        let mut class_list_expr: Option<&JSXExpressionContainer<'a>> = None;

        // Check for transition attributes
        let mut transition_name: Option<String> = None;
        let mut transition_animate: Option<String> = None;
        let mut transition_persist: Option<String> = None;

        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let name = get_jsx_attribute_name(&attr.name);
                if name == "class" {
                    if let Some(JSXAttributeValue::StringLiteral(lit)) = &attr.value {
                        static_class = Some(lit.value.as_str());
                    }
                } else if name == "class:list" {
                    if let Some(JSXAttributeValue::ExpressionContainer(expr)) = &attr.value {
                        class_list_expr = Some(expr);
                    }
                } else if name == "transition:name" {
                    transition_name = Some(Self::get_attr_value_string(attr));
                } else if name == "transition:animate" {
                    transition_animate = Some(Self::get_attr_value_string(attr));
                } else if name == "transition:persist" {
                    transition_persist = Some(Self::get_attr_value_string_or_empty(attr));
                } else if name == "transition:persist-props" {
                    // This is handled like transition:persist but also sets props
                    transition_persist = Some(Self::get_attr_value_string_or_empty(attr));
                }
            }
        }

        // If both class and class:list exist, merge them
        let has_merged_class = static_class.is_some() && class_list_expr.is_some();

        // Handle transition:persist - if there's a transition:name, use that for persist value
        // Otherwise use $$createTransitionScope
        if let Some(persist_val) = &transition_persist {
            if let Some(name_val) = &transition_name {
                // When transition:name is present, use the name value for persist (static attr)
                // Strip quotes if present for static attribute value
                let clean_val = name_val.trim_matches('"');
                self.print(&format!(" data-astro-transition-persist=\"{clean_val}\""));
            } else {
                // No transition:name, use $$createTransitionScope
                let hash = self.generate_transition_hash();
                if persist_val.is_empty() || *persist_val == "\"\"" {
                    // Just transition:persist without a value
                    self.print(&format!(
                        "${{{}({}({}, \"{}\"), \"data-astro-transition-persist\")}}",
                        runtime::ADD_ATTRIBUTE,
                        runtime::CREATE_TRANSITION_SCOPE,
                        runtime::RESULT,
                        hash
                    ));
                } else {
                    // transition:persist="value" or transition:persist={expr}
                    self.print(&format!(
                        "${{{}({}({}, \"{}\"), \"data-astro-transition-persist\")}}",
                        runtime::ADD_ATTRIBUTE,
                        runtime::CREATE_TRANSITION_SCOPE,
                        runtime::RESULT,
                        hash
                    ));
                }
            }
        }

        // Handle transition:name and transition:animate together
        if transition_name.is_some() || transition_animate.is_some() {
            let name_val = transition_name.unwrap_or_else(|| "\"\"".to_string());
            let animate_val = transition_animate.unwrap_or_else(|| "\"\"".to_string());
            let hash = self.generate_transition_hash();
            self.print(&format!(
                "${{{}({}({}, \"{}\", {}, {}), \"data-astro-transition-scope\")}}",
                runtime::ADD_ATTRIBUTE,
                runtime::RENDER_TRANSITION,
                runtime::RESULT,
                hash,
                animate_val,
                name_val
            ));
        }

        for attr in attrs {
            match attr {
                JSXAttributeItem::Attribute(attr) => {
                    let name = get_jsx_attribute_name(&attr.name);
                    // Skip set:html and set:text - handled separately
                    if name == "set:html" || name == "set:text" {
                        continue;
                    }
                    // Skip slot attribute if we're inside a conditional slot context
                    if self.skip_slot_attribute && name == "slot" {
                        continue;
                    }
                    // Skip is:inline and is:raw - these are Astro directives not HTML attributes
                    if name == "is:inline" || name == "is:raw" {
                        continue;
                    }
                    // Skip transition directives - already handled above
                    if name.starts_with("transition:") {
                        continue;
                    }
                    // Skip individual class if we're merging with class:list
                    if has_merged_class && name == "class" {
                        continue;
                    }
                    // Handle merged class:list
                    if has_merged_class && name == "class:list" {
                        if let (Some(static_val), Some(expr)) = (static_class, class_list_expr) {
                            self.print(&format!("${{{}([", runtime::ADD_ATTRIBUTE));
                            self.print(&format!("\"{}\"", escape_double_quotes(static_val)));
                            self.print(", ");
                            self.print_jsx_expression(&expr.expression);
                            self.print("], \"class:list\")}");
                        }
                        continue;
                    }
                    self.print_html_attribute(attr);
                }
                JSXAttributeItem::SpreadAttribute(spread) => {
                    self.print(&format!("${{{}(", runtime::SPREAD_ATTRIBUTES));
                    self.print_expression(&spread.argument);
                    self.print(")}");
                }
            }
        }
    }

    /// Get attribute value as a string representation for codegen
    fn get_attr_value_string(attr: &JSXAttribute<'a>) -> String {
        match &attr.value {
            Some(JSXAttributeValue::StringLiteral(lit)) => {
                format!("\"{}\"", escape_double_quotes(lit.value.as_str()))
            }
            Some(JSXAttributeValue::ExpressionContainer(expr)) => {
                if let Some(e) = expr.expression.as_expression() {
                    let mut codegen = Codegen::new();
                    e.print_expr(
                        &mut codegen,
                        oxc_syntax::precedence::Precedence::Lowest,
                        Context::default().with_typescript(),
                    );
                    let source = codegen.into_source_text();
                    // Template literals don't need parens, but other expressions do
                    if matches!(e, Expression::TemplateLiteral(_)) {
                        source
                    } else {
                        format!("({source})")
                    }
                } else {
                    "\"\"".to_string()
                }
            }
            _ => "\"\"".to_string(),
        }
    }

    /// Get attribute value as a string, or empty string if no value (for boolean attrs)
    fn get_attr_value_string_or_empty(attr: &JSXAttribute<'a>) -> String {
        match &attr.value {
            None => String::new(),
            _ => Self::get_attr_value_string(attr),
        }
    }

    /// Generate a hash for transition scope
    /// Go compiler uses: HashString(fmt.Sprintf("%s-%v", opts.Scope, i))
    /// where opts.Scope is the file hash and i is the element index
    fn generate_transition_hash(&self) -> String {
        use std::collections::hash_map::DefaultHasher;
        use std::hash::{Hash, Hasher};

        // Increment counter to get unique index
        let counter = self.transition_counter.get();
        self.transition_counter.set(counter + 1);

        // Hash the combination of source hash + counter (like Go's "%s-%v" format)
        let mut hasher = DefaultHasher::new();
        format!("{}-{}", self.source_hash, counter).hash(&mut hasher);
        let hash = hasher.finish();

        // Convert to base32-like lowercase string (8 chars)
        Self::to_base32_like(hash)
    }

    fn print_html_attribute(&mut self, attr: &JSXAttribute<'a>) {
        let name = get_jsx_attribute_name(&attr.name);

        match &attr.value {
            None => {
                // Boolean attribute
                self.print(" ");
                self.print(&name);
            }
            Some(value) => match value {
                JSXAttributeValue::StringLiteral(lit) => {
                    self.print(" ");
                    self.print(&name);
                    self.print("=\"");
                    self.print(&escape_html_attribute(lit.value.as_str()));
                    self.print("\"");
                }
                JSXAttributeValue::ExpressionContainer(expr) => {
                    // Dynamic attribute
                    self.print(&format!("${{{}(", runtime::ADD_ATTRIBUTE));
                    self.print_jsx_expression(&expr.expression);
                    self.print(", \"");
                    self.print(&name);
                    self.print("\")}");
                }
                JSXAttributeValue::Element(el) => {
                    // JSX element as attribute value (rare)
                    self.print(" ");
                    self.print(&name);
                    self.print("=\"");
                    self.print_jsx_element(el);
                    self.print("\"");
                }
                JSXAttributeValue::Fragment(frag) => {
                    self.print(" ");
                    self.print(&name);
                    self.print("=\"");
                    self.print_jsx_fragment(frag);
                    self.print("\"");
                }
            },
        }
    }

    /// Check if a JSX child has meaningful content (not just whitespace or empty expressions)
    fn jsx_child_has_content(child: &JSXChild<'a>) -> bool {
        match child {
            JSXChild::Text(text) => !text.value.trim().is_empty(),
            JSXChild::ExpressionContainer(expr) => {
                !matches!(expr.expression, JSXExpression::EmptyExpression(_))
            }
            JSXChild::Element(_)
            | JSXChild::Fragment(_)
            | JSXChild::Spread(_)
            | JSXChild::AstroComment(_) => true,
            JSXChild::AstroScript(_) | JSXChild::AstroDoctype(_) => false,
        }
    }

    /// Print all children as a single default slot, preserving slot attributes.
    /// Used for custom elements (web components) where the browser handles slots.
    fn print_component_default_slot_only(&mut self, children: &[JSXChild<'a>]) {
        let async_prefix = self.get_async_prefix();
        let slot_params = self.get_slot_params();
        self.print("{\"default\": ");
        self.print(&format!("{async_prefix}{slot_params}"));
        self.print(runtime::RENDER);
        self.print("`");

        // DO NOT set skip_slot_attribute - we want to preserve slot="..." for custom elements
        for child in children {
            // Skip HTML comments in slots if configured (matches Go compiler behavior)
            if self.options.strip_slot_comments && matches!(child, JSXChild::AstroComment(_)) {
                continue;
            }
            self.print_jsx_child(child);
        }

        self.print("`,}");
    }

    fn print_component_slots(&mut self, children: &[JSXChild<'a>]) {
        // Categorize children into:
        // 1. default_children - children without slot attribute
        // 2. named_slots - direct elements with slot="name"
        // 3. expression_slots - expressions containing single slotted element
        // 4. conditional_slots - expressions with multiple different slots (need $$mergeSlots)
        let mut default_children: Vec<&JSXChild<'a>> = Vec::new();
        let mut named_slots: Vec<(&str, Vec<&JSXChild<'a>>)> = Vec::new();
        let mut expression_slots: Vec<(&str, &JSXChild<'a>)> = Vec::new();
        let mut conditional_slots: Vec<&JSXExpressionContainer<'a>> = Vec::new();

        // Also track dynamic slots separately (elements with slot={expr})
        let mut dynamic_slots: Vec<(String, Vec<&JSXChild<'a>>)> = Vec::new();

        for child in children {
            // Skip HTML comments in slots if configured (matches Go compiler behavior)
            // The Go compiler explicitly excludes CommentNode from slots:
            // "Only slot ElementNodes (except expressions containing only comments) or non-empty TextNodes!
            //  CommentNode, JSX comments and others should not be slotted"
            if self.options.strip_slot_comments && matches!(child, JSXChild::AstroComment(_)) {
                continue;
            }

            match child {
                JSXChild::Element(el) => {
                    // Check for slot attribute on direct element children
                    match get_slot_attribute_value(&el.opening_element.attributes) {
                        Some(SlotValue::Static(slot_name)) => {
                            // Static slot: slot="name"
                            if let Some((_, slot_children)) =
                                named_slots.iter_mut().find(|(name, _)| *name == slot_name)
                            {
                                slot_children.push(child);
                            } else {
                                named_slots.push((slot_name.leak(), vec![child]));
                            }
                        }
                        Some(SlotValue::Dynamic(expr)) => {
                            // Dynamic slot: slot={expr}
                            dynamic_slots.push((expr, vec![child]));
                        }
                        None => {
                            default_children.push(child);
                        }
                    }
                }
                JSXChild::ExpressionContainer(expr) => {
                    // Check if expression contains slotted elements
                    match extract_slots_from_expression(&expr.expression) {
                        ExpressionSlotInfo::None => {
                            // No slots in expression - goes to default slot
                            default_children.push(child);
                        }
                        ExpressionSlotInfo::Single(slot_name) => {
                            // Single slot found - put expression in that named slot
                            expression_slots.push((slot_name, child));
                        }
                        ExpressionSlotInfo::Multiple(_) => {
                            // Multiple slots - needs $$mergeSlots
                            conditional_slots.push(expr);
                        }
                    }
                }
                _ => {
                    default_children.push(child);
                }
            }
        }

        // Determine if we need $$mergeSlots wrapper
        let needs_merge_slots = !conditional_slots.is_empty();

        if needs_merge_slots {
            self.print(runtime::MERGE_SLOTS);
            self.print("(");
        }

        self.print("{");

        // Print default slot only if there are children with actual content
        let has_meaningful_content =
            default_children.iter().any(|c| Self::jsx_child_has_content(c));
        if has_meaningful_content {
            let async_prefix = self.get_async_prefix();
            let slot_params = self.get_slot_params();
            self.print(&format!("\"default\": {async_prefix}{slot_params}"));
            self.print(runtime::RENDER);
            self.print("`");
            for child in &default_children {
                self.print_jsx_child(child);
            }
            self.print("`,");
        }

        // Print named slots (direct elements with slot attribute)
        for (name, slot_children) in &named_slots {
            let async_prefix = self.get_async_prefix();
            let slot_params = self.get_slot_params();
            self.print("\"");
            self.print(&escape_double_quotes(name));
            self.print(&format!("\": {async_prefix}{slot_params}"));
            self.print(runtime::RENDER);
            self.print("`");
            // Skip slot attribute when printing these children
            self.skip_slot_attribute = true;
            for child in slot_children {
                self.print_jsx_child(child);
            }
            self.skip_slot_attribute = false;
            self.print("`,");
        }

        // Print expression slots (expressions with single slot)
        for (name, child) in &expression_slots {
            let async_prefix = self.get_async_prefix();
            let slot_params = self.get_slot_params();
            self.print("\"");
            self.print(&escape_double_quotes(name));
            self.print(&format!("\": {async_prefix}{slot_params}"));
            self.print(runtime::RENDER);
            self.print("`");
            // Skip slot attribute when printing the child
            self.skip_slot_attribute = true;
            self.print_jsx_child(child);
            self.skip_slot_attribute = false;
            self.print("`,");
        }

        // Print dynamic slots (elements with slot={expr}) using computed property syntax
        for (expr, slot_children) in &dynamic_slots {
            let async_prefix = self.get_async_prefix();
            let slot_params = self.get_slot_params();
            // Use computed property syntax: [expr]: () => ...
            self.print("[");
            self.print(expr);
            self.print(&format!("]: {async_prefix}{slot_params}"));
            self.print(runtime::RENDER);
            self.print("`");
            // Skip slot attribute when printing these children
            self.skip_slot_attribute = true;
            for child in slot_children {
                self.print_jsx_child(child);
            }
            self.skip_slot_attribute = false;
            self.print("`,");
        }

        self.print("}");

        // Print conditional slots (expressions with multiple slots) for $$mergeSlots
        for expr in &conditional_slots {
            self.print(",");
            self.print_conditional_slot_expression(expr);
        }

        if needs_merge_slots {
            self.print(")");
        }
    }

    /// Print an expression with multiple conditional slots for $$mergeSlots.
    /// Transforms: `cond ? <div slot="a"> : <div slot="b">`
    /// Into: `cond ? {"a": () => $$render`<div>`} : {"b": () => $$render`<div>`}`
    fn print_conditional_slot_expression(&mut self, expr: &JSXExpressionContainer<'a>) {
        self.print_conditional_slot_expr(&expr.expression);
    }

    fn print_conditional_slot_expr(&mut self, expr: &JSXExpression<'a>) {
        match expr {
            JSXExpression::ConditionalExpression(cond) => {
                self.print_expression(&cond.test);
                self.print(" ? ");
                self.print_conditional_slot_branch(&cond.consequent);
                self.print(" : ");
                self.print_conditional_slot_branch(&cond.alternate);
            }
            JSXExpression::ArrowFunctionExpression(arrow) => {
                // Arrow function with slot returns: () => { switch(x) { case 'a': return <div slot="a">A</div> } }
                self.print_slot_aware_arrow_function(arrow);
            }
            _ => {
                if let Some(inner) = expr.as_expression() {
                    self.print_conditional_slot_branch_expr(inner);
                } else {
                    self.print_jsx_expression(expr);
                }
            }
        }
    }

    /// Print an expression that might contain conditional slot returns.
    /// This dispatches to the slot-aware versions for arrows/functions.
    fn print_conditional_slot_branch_expr(&mut self, expr: &Expression<'a>) {
        match expr {
            Expression::ArrowFunctionExpression(arrow) => {
                self.print_slot_aware_arrow_function(arrow);
            }
            Expression::ConditionalExpression(cond) => {
                self.print_expression(&cond.test);
                self.print(" ? ");
                self.print_conditional_slot_branch(&cond.consequent);
                self.print(" : ");
                self.print_conditional_slot_branch(&cond.alternate);
            }
            _ => {
                self.print_expression(expr);
            }
        }
    }

    /// Print an arrow function where return statements may contain slotted JSX.
    /// Transforms `return <div slot="a">A</div>` into `return {"a": () => $$render\`<div>A</div>\`}`
    fn print_slot_aware_arrow_function(
        &mut self,
        arrow: &oxc_ast::ast::ArrowFunctionExpression<'a>,
    ) {
        if arrow.r#async {
            self.print("async ");
        }
        // Print parameters
        let needs_parens = arrow.params.items.len() != 1
            || arrow.params.rest.is_some()
            || !matches!(
                arrow.params.items.first().map(|p| &p.pattern),
                Some(oxc_ast::ast::BindingPattern::BindingIdentifier(_))
            );

        if needs_parens {
            self.print("(");
        }
        let mut first = true;
        for param in &arrow.params.items {
            if !first {
                self.print(", ");
            }
            first = false;
            self.print_binding_pattern(&param.pattern);
        }
        if let Some(rest) = &arrow.params.rest {
            if !first {
                self.print(", ");
            }
            self.print("...");
            self.print_binding_pattern(&rest.rest.argument);
        }
        if needs_parens {
            self.print(")");
        }
        self.print(" => ");

        if arrow.expression {
            // Expression body
            if let Some(expr) = arrow.body.statements.first()
                && let oxc_ast::ast::Statement::ExpressionStatement(expr_stmt) = expr
            {
                self.print_conditional_slot_branch(&expr_stmt.expression);
            }
        } else {
            // Block body with slot-aware statements
            self.print("{\n");
            for stmt in &arrow.body.statements {
                self.print_slot_aware_statement(stmt);
            }
            self.print("}");
        }
    }

    /// Print a statement where return values may contain slotted JSX.
    fn print_slot_aware_statement(&mut self, stmt: &oxc_ast::ast::Statement<'a>) {
        use oxc_ast::ast::Statement;
        match stmt {
            Statement::ReturnStatement(ret) => {
                self.print("return ");
                if let Some(arg) = &ret.argument {
                    self.print_conditional_slot_branch(arg);
                }
                self.print("\n");
            }
            Statement::SwitchStatement(switch_stmt) => {
                self.print("switch (");
                self.print_expression(&switch_stmt.discriminant);
                self.print(") {\n");
                for case in &switch_stmt.cases {
                    if let Some(test) = &case.test {
                        self.print("case ");
                        self.print_expression(test);
                        self.print(":");
                    } else {
                        self.print("default:");
                    }
                    for s in &case.consequent {
                        self.print_slot_aware_statement(s);
                    }
                    self.print("\n");
                }
                self.print("}");
            }
            Statement::BlockStatement(block) => {
                self.print("{\n");
                for s in &block.body {
                    self.print_slot_aware_statement(s);
                }
                self.print("}");
            }
            Statement::IfStatement(if_stmt) => {
                self.print("if (");
                self.print_expression(&if_stmt.test);
                self.print(") ");
                self.print_slot_aware_statement(&if_stmt.consequent);
                if let Some(alt) = &if_stmt.alternate {
                    self.print(" else ");
                    self.print_slot_aware_statement(alt);
                }
            }
            _ => {
                let mut codegen = Codegen::new();
                stmt.print(&mut codegen, Context::default().with_typescript());
                let code = codegen.into_source_text();
                self.print(&code);
                self.print("\n");
            }
        }
    }

    fn print_conditional_slot_branch(&mut self, expr: &Expression<'a>) {
        match expr {
            Expression::JSXElement(el) => {
                // Extract slot name
                if let Some(slot_name) = get_slot_attribute(&el.opening_element.attributes) {
                    let async_prefix = self.get_async_prefix();
                    let slot_params = self.get_slot_params();
                    self.print("{\"");
                    self.print(&escape_double_quotes(slot_name));
                    self.print(&format!("\": {async_prefix}{slot_params}"));
                    self.print(runtime::RENDER);
                    self.print("`");
                    self.skip_slot_attribute = true;
                    self.print_jsx_element(el);
                    self.skip_slot_attribute = false;
                    self.print("`}");
                } else {
                    // No slot attribute - print as default
                    let async_prefix = self.get_async_prefix();
                    let slot_params = self.get_slot_params();
                    self.print("{\"default\": ");
                    self.print(&format!("{async_prefix}{slot_params}"));
                    self.print(runtime::RENDER);
                    self.print("`");
                    self.print_jsx_element(el);
                    self.print("`}");
                }
            }
            Expression::ConditionalExpression(cond) => {
                // Nested ternary
                self.print_expression(&cond.test);
                self.print(" ? ");
                self.print_conditional_slot_branch(&cond.consequent);
                self.print(" : ");
                self.print_conditional_slot_branch(&cond.alternate);
            }
            _ => {
                // Other expression types - use default codegen
                self.print_expression(expr);
            }
        }
    }

    fn print_jsx_fragment(&mut self, frag: &JSXFragment<'a>) {
        // Render fragment using $$renderComponent with Fragment
        let async_prefix = self.get_async_prefix();
        let slot_params = self.get_slot_params();
        self.print("${");
        self.print(runtime::RENDER_COMPONENT);
        self.print("(");
        self.print(runtime::RESULT);
        self.print(",\"Fragment\",");
        self.print(runtime::FRAGMENT);
        self.print(&format!(",{{}},{{\"default\": {async_prefix}{slot_params}"));
        self.print(runtime::RENDER);
        self.print("`");
        for child in &frag.children {
            self.print_jsx_child(child);
        }
        self.print("`,})}");
    }

    fn print_jsx_expression_container(&mut self, expr: &JSXExpressionContainer<'a>) {
        // Check if this is a comment-only expression that should be stripped
        if let JSXExpression::EmptyExpression(empty) = &expr.expression {
            let content = &self.source_text[empty.span.start as usize..empty.span.end as usize];
            let trimmed = content.trim();
            // Comment-only expressions should be stripped entirely
            if !trimmed.is_empty() {
                return;
            }
        }

        self.print("${");
        self.print_jsx_expression(&expr.expression);
        self.print("}");
    }

    fn print_jsx_expression(&mut self, expr: &JSXExpression<'a>) {
        match expr {
            JSXExpression::EmptyExpression(_) => {
                // Empty {} or whitespace-only {   } renders as (void 0) to match Go compiler
                // Comment-only expressions are already filtered out in print_jsx_expression_container
                self.print("(void 0)");
            }
            JSXExpression::Identifier(ident) => {
                self.print(ident.name.as_str());
            }
            other => {
                // Use regular codegen for complex expressions
                if let Some(expr) = other.as_expression() {
                    self.print_expression(expr);
                }
            }
        }
    }

    fn print_jsx_spread_child(&mut self, spread: &JSXSpreadChild<'a>) {
        self.print("${");
        self.print_expression(&spread.expression);
        self.print("}");
    }

    fn print_expression(&mut self, expr: &Expression<'a>) {
        // Handle expressions that may contain JSX
        // JSX inside expressions needs to be wrapped in $$render`...`
        match expr {
            Expression::JSXElement(el) => {
                // Wrap JSX in $$render`...`
                self.print(runtime::RENDER);
                self.print("`");
                self.print_jsx_element(el);
                self.print("`");
            }
            Expression::JSXFragment(frag) => {
                // Check if this is an explicit fragment (<>...</>) or implicit (multiple JSX siblings)
                // Implicit fragments have zero-length opening_fragment span
                let is_explicit_fragment = !frag.opening_fragment.span.is_empty();

                if is_explicit_fragment {
                    // Explicit <>...</> syntax gets wrapped in $$renderComponent with Fragment
                    let slot_params = self.get_slot_params();
                    self.print(runtime::RENDER);
                    self.print("`${");
                    self.print(runtime::RENDER_COMPONENT);
                    self.print(&format!("($$result,\"Fragment\",Fragment,{{}},{{\"default\":{slot_params}"));
                    self.print(runtime::RENDER);
                    self.print("`");
                    for child in &frag.children {
                        self.print_jsx_child(child);
                    }
                    self.print("`,})}`");
                } else {
                    // Implicit fragments (multiple JSX siblings) are just wrapped in $$render`...`
                    self.print(runtime::RENDER);
                    self.print("`");
                    for child in &frag.children {
                        self.print_jsx_child(child);
                    }
                    self.print("`");
                }
            }
            Expression::ConditionalExpression(cond) => {
                // Recursively handle ternary: test ? consequent : alternate
                self.print_expression(&cond.test);
                self.print(" ? ");
                self.print_expression(&cond.consequent);
                self.print(" : ");
                self.print_expression(&cond.alternate);
            }
            Expression::LogicalExpression(logic) => {
                // Recursively handle && and ||
                self.print_expression(&logic.left);
                self.print(match logic.operator {
                    oxc_ast::ast::LogicalOperator::And => " && ",
                    oxc_ast::ast::LogicalOperator::Or => " || ",
                    oxc_ast::ast::LogicalOperator::Coalesce => " ?? ",
                });
                self.print_expression(&logic.right);
            }
            Expression::ParenthesizedExpression(paren) => {
                self.print("(");
                self.print_expression(&paren.expression);
                self.print(")");
            }
            Expression::ChainExpression(chain) => {
                // Handle optional chaining like arr?.map(x => <JSX>)
                self.print_chain_expression(chain);
            }
            Expression::CallExpression(call) => {
                // Handle call expressions like arr.map(x => <JSX>)
                self.print_call_expression(call);
            }
            Expression::ArrowFunctionExpression(arrow) => {
                // Handle arrow functions that may return JSX
                self.print_arrow_function(arrow);
            }
            _ => {
                // For all other expressions, use regular codegen
                let mut codegen = Codegen::new();
                expr.print_expr(
                    &mut codegen,
                    oxc_syntax::precedence::Precedence::Lowest,
                    Context::default().with_typescript(),
                );
                let code = codegen.into_source_text();
                self.print(&code);
            }
        }
    }

    fn print_chain_expression(&mut self, chain: &oxc_ast::ast::ChainExpression<'a>) {
        match &chain.expression {
            oxc_ast::ast::ChainElement::CallExpression(call) => {
                self.print_call_expression(call);
            }
            oxc_ast::ast::ChainElement::StaticMemberExpression(member) => {
                self.print_expression(&member.object);
                self.print(if member.optional { "?." } else { "." });
                self.print(member.property.name.as_str());
            }
            oxc_ast::ast::ChainElement::ComputedMemberExpression(member) => {
                self.print_expression(&member.object);
                self.print(if member.optional { "?.[" } else { "[" });
                self.print_expression(&member.expression);
                self.print("]");
            }
            _ => {
                // TSNonNullExpression, PrivateFieldExpression - use source text fallback
                let start = chain.span.start as usize;
                let end = chain.span.end as usize;
                if start < self.source_text.len() && end <= self.source_text.len() {
                    self.print(&self.source_text[start..end]);
                }
            }
        }
    }

    fn print_call_expression(&mut self, call: &oxc_ast::ast::CallExpression<'a>) {
        // Print callee
        match &call.callee {
            Expression::StaticMemberExpression(member) => {
                self.print_expression(&member.object);
                self.print(if member.optional { "?." } else { "." });
                self.print(member.property.name.as_str());
            }
            Expression::ComputedMemberExpression(member) => {
                self.print_expression(&member.object);
                self.print(if member.optional { "?.[" } else { "[" });
                self.print_expression(&member.expression);
                self.print("]");
            }
            other => {
                self.print_expression(other);
            }
        }
        // Print optional call syntax
        if call.optional {
            self.print("?.");
        }
        // Print arguments
        self.print("(");
        let mut first = true;
        for arg in &call.arguments {
            if !first {
                self.print(", ");
            }
            first = false;
            match arg {
                oxc_ast::ast::Argument::SpreadElement(spread) => {
                    self.print("...");
                    self.print_expression(&spread.argument);
                }
                _ => {
                    if let Some(expr) = arg.as_expression() {
                        self.print_expression(expr);
                    }
                }
            }
        }
        self.print(")");
    }

    fn print_arrow_function(&mut self, arrow: &oxc_ast::ast::ArrowFunctionExpression<'a>) {
        if arrow.r#async {
            self.print("async ");
        }
        // Print parameters
        // Single simple identifier param doesn't need parens, but destructuring patterns do
        let needs_parens = arrow.params.items.len() != 1
            || arrow.params.rest.is_some()
            || !matches!(
                arrow.params.items.first().map(|p| &p.pattern),
                Some(oxc_ast::ast::BindingPattern::BindingIdentifier(_))
            );

        if needs_parens {
            self.print("(");
        }

        let mut first = true;
        for param in &arrow.params.items {
            if !first {
                self.print(", ");
            }
            first = false;
            self.print_binding_pattern(&param.pattern);
        }
        if let Some(rest) = &arrow.params.rest {
            if !first {
                self.print(", ");
            }
            self.print("...");
            self.print_binding_pattern(&rest.rest.argument);
        }

        if needs_parens {
            self.print(")");
        }
        self.print(" => ");

        // Print body
        if arrow.expression {
            // Expression body - may contain JSX
            if let Some(expr) = arrow.body.statements.first()
                && let oxc_ast::ast::Statement::ExpressionStatement(expr_stmt) = expr
            {
                self.print_expression(&expr_stmt.expression);
            }
        } else {
            // Block body - need to handle JSX in return statements
            self.print_jsx_aware_function_body(&arrow.body);
        }
    }

    fn print_jsx_aware_function_body(&mut self, body: &oxc_ast::ast::FunctionBody<'a>) {
        self.print("{\n");
        for stmt in &body.statements {
            self.print_jsx_aware_statement(stmt);
        }
        self.print("}");
    }

    fn print_jsx_aware_statement(&mut self, stmt: &oxc_ast::ast::Statement<'a>) {
        use oxc_ast::ast::Statement;
        match stmt {
            Statement::ReturnStatement(ret) => {
                self.print("\treturn ");
                if let Some(arg) = &ret.argument {
                    self.print_expression(arg);
                }
                self.print(";\n");
            }
            Statement::ExpressionStatement(expr_stmt) => {
                self.print_expression(&expr_stmt.expression);
                self.print(";\n");
            }
            Statement::VariableDeclaration(decl) => {
                // Use regular codegen for variable declarations
                let mut codegen = Codegen::new();
                decl.print(&mut codegen, Context::default().with_typescript());
                let code = codegen.into_source_text();
                self.print(&code);
                self.print("\n");
            }
            Statement::IfStatement(if_stmt) => {
                self.print("if (");
                self.print_expression(&if_stmt.test);
                self.print(") ");
                self.print_jsx_aware_statement(&if_stmt.consequent);
                if let Some(alt) = &if_stmt.alternate {
                    self.print(" else ");
                    self.print_jsx_aware_statement(alt);
                }
            }
            Statement::BlockStatement(block) => {
                self.print("{\n");
                for s in &block.body {
                    self.print_jsx_aware_statement(s);
                }
                self.print("}");
            }
            Statement::SwitchStatement(switch_stmt) => {
                self.print("switch (");
                self.print_expression(&switch_stmt.discriminant);
                self.print(") {\n");
                for case in &switch_stmt.cases {
                    if let Some(test) = &case.test {
                        self.print("case ");
                        self.print_expression(test);
                        self.print(":");
                    } else {
                        self.print("default:");
                    }
                    for s in &case.consequent {
                        self.print_jsx_aware_statement(s);
                    }
                    self.print("\n");
                }
                self.print("}");
            }
            _ => {
                // For other statements, use regular codegen
                let mut codegen = Codegen::new();
                stmt.print(&mut codegen, Context::default().with_typescript());
                let code = codegen.into_source_text();
                self.print(&code);
                self.print("\n");
            }
        }
    }

    fn print_binding_pattern(&mut self, pattern: &oxc_ast::ast::BindingPattern<'a>) {
        if let oxc_ast::ast::BindingPattern::BindingIdentifier(ident) = pattern {
            self.print(ident.name.as_str());
        } else {
            // For complex patterns, use regular codegen
            let mut codegen = Codegen::new();
            pattern.print(&mut codegen, Context::default().with_typescript());
            let code = codegen.into_source_text();
            self.print(&code);
        }
    }

    fn extract_hydration_info(attrs: &[JSXAttributeItem<'a>]) -> HydrationInfo {
        let mut info = HydrationInfo::default();

        for attr in attrs {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let name = get_jsx_attribute_name(&attr.name);

                if let Some(directive) = name.strip_prefix("client:") {
                    info.directive = Some(directive.to_string());

                    // Hydration directives are tracked by the scanner

                    // Extract value for client:only="framework"
                    if directive == "only" {
                        if let Some(JSXAttributeValue::StringLiteral(lit)) = &attr.value {
                            info.framework = Some(lit.value.to_string());
                        }
                        info.is_client_only = true;
                    }
                }
            }
        }

        info
    }
}

/// Information about component hydration directives.
#[derive(Default)]
struct HydrationInfo {
    /// The hydration directive (e.g., "load", "visible", "only")
    directive: Option<String>,
    /// For client:only, the framework name
    framework: Option<String>,
    /// Whether this is a client:only component
    is_client_only: bool,
    /// Component import path (for hydration)
    component_path: Option<String>,
    /// Component export name (for hydration)
    component_export: Option<String>,
}

// Helper functions

/// Check if an import specifier refers to a CSS file.
/// Matches the Go compiler's `styleModuleSpecExp` regex.
fn is_css_specifier(specifier: &str) -> bool {
    matches!(
        specifier.rsplit('.').next(),
        Some("css" | "pcss" | "postcss" | "sass" | "scss" | "styl" | "stylus" | "less")
    )
}

fn is_void_element(name: &str) -> bool {
    matches!(
        name,
        "area"
            | "base"
            | "br"
            | "col"
            | "embed"
            | "hr"
            | "img"
            | "input"
            | "link"
            | "meta"
            | "param"
            | "selectedcontent" // HTML customizable select element
            | "source"
            | "track"
            | "wbr"
    )
}

/// Elements that can appear in `<head>` and should NOT trigger `$$maybeRenderHead`.
/// This matches the Go compiler's skip list in print-to-js.go:
/// `atom.Html, atom.Head, atom.Base, atom.Basefont, atom.Bgsound, atom.Link,
///  atom.Meta, atom.Noframes, atom.Script, atom.Style, atom.Template, atom.Title`
fn is_head_element(name: &str) -> bool {
    matches!(
        name,
        "html"
            | "head"
            | "base"
            | "basefont"
            | "bgsound"
            | "link"
            | "meta"
            | "noframes"
            | "script"
            | "style"
            | "template"
            | "title"
    )
}

/// Represents a slot attribute value - either a static string or a dynamic expression
#[derive(Debug, Clone)]
enum SlotValue {
    /// Static slot name like slot="header"
    Static(String),
    /// Dynamic slot name like slot={name} - stores the expression as a string
    Dynamic(String),
}

fn get_slot_attribute<'a>(attrs: &'a [JSXAttributeItem<'a>]) -> Option<&'a str> {
    for attr in attrs {
        if let JSXAttributeItem::Attribute(attr) = attr
            && let JSXAttributeName::Identifier(ident) = &attr.name
            && ident.name.as_str() == "slot"
            && let Some(JSXAttributeValue::StringLiteral(lit)) = &attr.value
        {
            return Some(lit.value.as_str());
        }
    }
    None
}

fn get_slot_attribute_value(attrs: &[JSXAttributeItem<'_>]) -> Option<SlotValue> {
    for attr in attrs {
        if let JSXAttributeItem::Attribute(attr) = attr
            && let JSXAttributeName::Identifier(ident) = &attr.name
            && ident.name.as_str() == "slot"
        {
            match &attr.value {
                Some(JSXAttributeValue::StringLiteral(lit)) => {
                    return Some(SlotValue::Static(lit.value.to_string()));
                }
                Some(JSXAttributeValue::ExpressionContainer(expr)) => {
                    if let Some(e) = expr.expression.as_expression() {
                        let mut codegen = Codegen::new();
                        e.print_expr(
                            &mut codegen,
                            oxc_syntax::precedence::Precedence::Lowest,
                            Context::default().with_typescript(),
                        );
                        return Some(SlotValue::Dynamic(codegen.into_source_text()));
                    }
                }
                _ => {}
            }
        }
    }
    None
}

/// Information about slots found within an expression container.
#[derive(Debug)]
#[expect(dead_code)] // Multiple variant is used for matching but contents aren't accessed yet
enum ExpressionSlotInfo<'a> {
    /// No slotted elements found - treat as default slot
    None,
    /// Single slot found - use that slot name for the entire expression
    Single(&'a str),
    /// Multiple different slots found - requires $$mergeSlots
    Multiple(Vec<&'a str>),
}

/// Extract slot information from a JSX expression.
/// Recursively searches for JSX elements with slot attributes.
fn extract_slots_from_expression<'a>(expr: &'a JSXExpression<'a>) -> ExpressionSlotInfo<'a> {
    let mut slots = Vec::new();
    collect_slots_from_expression(expr, &mut slots);

    match slots.len() {
        0 => ExpressionSlotInfo::None,
        1 => ExpressionSlotInfo::Single(slots[0]),
        _ => {
            // Check if all slots are the same
            if slots.iter().all(|s| *s == slots[0]) {
                ExpressionSlotInfo::Single(slots[0])
            } else {
                ExpressionSlotInfo::Multiple(slots)
            }
        }
    }
}

/// Recursively collect slot names from an expression.
fn collect_slots_from_expression<'a>(expr: &'a JSXExpression<'a>, slots: &mut Vec<&'a str>) {
    match expr {
        JSXExpression::JSXElement(el) => {
            if let Some(slot_name) = get_slot_attribute(&el.opening_element.attributes) {
                slots.push(slot_name);
            }
        }
        JSXExpression::JSXFragment(frag) => {
            for child in &frag.children {
                collect_slots_from_child(child, slots);
            }
        }
        JSXExpression::ConditionalExpression(cond) => {
            collect_slots_from_inner_expression(&cond.consequent, slots);
            collect_slots_from_inner_expression(&cond.alternate, slots);
        }
        JSXExpression::LogicalExpression(logic) => {
            // For &&/|| expressions, the JSX is typically on the right side
            collect_slots_from_inner_expression(&logic.right, slots);
        }
        JSXExpression::ParenthesizedExpression(paren) => {
            collect_slots_from_inner_expression(&paren.expression, slots);
        }
        JSXExpression::ArrowFunctionExpression(arrow) => {
            // Look for slots inside arrow function bodies (e.g., switch returning slotted JSX)
            collect_slots_from_function_body(&arrow.body, slots);
        }
        _ => {}
    }
}

/// Collect slots from an inner Expression (not JSXExpression).
fn collect_slots_from_inner_expression<'a>(expr: &'a Expression<'a>, slots: &mut Vec<&'a str>) {
    match expr {
        Expression::JSXElement(el) => {
            if let Some(slot_name) = get_slot_attribute(&el.opening_element.attributes) {
                slots.push(slot_name);
            }
        }
        Expression::JSXFragment(frag) => {
            for child in &frag.children {
                collect_slots_from_child(child, slots);
            }
        }
        Expression::ConditionalExpression(cond) => {
            collect_slots_from_inner_expression(&cond.consequent, slots);
            collect_slots_from_inner_expression(&cond.alternate, slots);
        }
        Expression::LogicalExpression(logic) => {
            collect_slots_from_inner_expression(&logic.right, slots);
        }
        Expression::ParenthesizedExpression(paren) => {
            collect_slots_from_inner_expression(&paren.expression, slots);
        }
        Expression::ArrowFunctionExpression(arrow) => {
            collect_slots_from_function_body(&arrow.body, slots);
        }
        _ => {}
    }
}

/// Collect slot names from return statements inside a function body.
fn collect_slots_from_function_body<'a>(
    body: &'a oxc_ast::ast::FunctionBody<'a>,
    slots: &mut Vec<&'a str>,
) {
    for stmt in &body.statements {
        collect_slots_from_statement(stmt, slots);
    }
}

/// Recursively collect slot names from statements (looking into return, switch, if, block).
fn collect_slots_from_statement<'a>(
    stmt: &'a oxc_ast::ast::Statement<'a>,
    slots: &mut Vec<&'a str>,
) {
    use oxc_ast::ast::Statement;
    match stmt {
        Statement::ReturnStatement(ret) => {
            if let Some(arg) = &ret.argument {
                collect_slots_from_inner_expression(arg, slots);
            }
        }
        Statement::SwitchStatement(switch_stmt) => {
            for case in &switch_stmt.cases {
                for s in &case.consequent {
                    collect_slots_from_statement(s, slots);
                }
            }
        }
        Statement::BlockStatement(block) => {
            for s in &block.body {
                collect_slots_from_statement(s, slots);
            }
        }
        Statement::IfStatement(if_stmt) => {
            collect_slots_from_statement(&if_stmt.consequent, slots);
            if let Some(alt) = &if_stmt.alternate {
                collect_slots_from_statement(alt, slots);
            }
        }
        _ => {}
    }
}

/// Collect slots from a JSX child.
fn collect_slots_from_child<'a>(child: &'a JSXChild<'a>, slots: &mut Vec<&'a str>) {
    match child {
        JSXChild::Element(el) => {
            if let Some(slot_name) = get_slot_attribute(&el.opening_element.attributes) {
                slots.push(slot_name);
            }
        }
        JSXChild::Fragment(frag) => {
            for child in &frag.children {
                collect_slots_from_child(child, slots);
            }
        }
        JSXChild::ExpressionContainer(expr) => {
            collect_slots_from_expression(&expr.expression, slots);
        }
        _ => {}
    }
}

fn escape_template_literal(s: &str) -> String {
    let mut result = String::with_capacity(s.len());
    let mut chars = s.chars().peekable();

    while let Some(c) = chars.next() {
        match c {
            '`' => result.push_str("\\`"),
            '$' if chars.peek() == Some(&'{') => {
                result.push_str("\\$");
            }
            '\\' => result.push_str("\\\\"),
            _ => result.push(c),
        }
    }

    result
}

fn escape_double_quotes(s: &str) -> String {
    s.cow_replace('"', "\\\"").into_owned()
}

fn escape_single_quote(s: &str) -> String {
    s.cow_replace('\'', "\\'").into_owned()
}

fn escape_html_attribute(s: &str) -> String {
    // Escape template literal syntax since we're inside a template literal
    let s = s.cow_replace('`', "\\`");
    let s = s.cow_replace("${", "\\${");
    // Escape HTML entities, but preserve valid HTML entities
    let s = escape_ampersands(&s);
    let s = s.cow_replace('"', "&quot;");
    let s = s.cow_replace('<', "&lt;");
    s.cow_replace('>', "&gt;").into_owned()
}

/// Escape ampersands, but preserve valid HTML entities like &#x22; or &quot;
fn escape_ampersands(s: &str) -> std::borrow::Cow<'_, str> {
    if !s.contains('&') {
        return std::borrow::Cow::Borrowed(s);
    }

    let mut result = String::with_capacity(s.len());
    let chars: Vec<char> = s.chars().collect();
    let mut i = 0;

    while i < chars.len() {
        if chars[i] == '&' {
            // Check if this is part of a valid HTML entity
            let remaining: String = chars[i..].iter().collect();
            if is_html_entity_start(&remaining) {
                // Keep the & as-is (part of an entity)
                result.push('&');
            } else {
                // Escape the &
                result.push_str("&amp;");
            }
        } else {
            result.push(chars[i]);
        }
        i += 1;
    }

    std::borrow::Cow::Owned(result)
}

/// Decode HTML entities in a string.
/// Handles numeric entities like &#x3C; (hex) and &#60; (decimal)
/// and common named entities like &lt; &gt; &amp; &quot; &apos;
fn decode_html_entities(s: &str) -> String {
    let mut result = String::with_capacity(s.len());
    let mut chars = s.chars().peekable();

    while let Some(c) = chars.next() {
        if c == '&' {
            // Try to parse an entity
            let mut entity = String::new();
            entity.push(c);

            while let Some(&next) = chars.peek() {
                entity.push(next);
                chars.next();
                if next == ';' {
                    break;
                }
                // Stop if we hit a non-entity character
                if !next.is_ascii_alphanumeric() && next != '#' {
                    break;
                }
            }

            // Try to decode the entity
            if let Some(decoded) = decode_entity(&entity) {
                result.push(decoded);
            } else {
                // Not a valid entity, keep as-is
                result.push_str(&entity);
            }
        } else {
            result.push(c);
        }
    }

    result
}

/// Decode a single HTML entity
fn decode_entity(entity: &str) -> Option<char> {
    if !entity.starts_with('&') || !entity.ends_with(';') {
        return None;
    }

    let inner = &entity[1..entity.len() - 1];

    // Numeric entities
    if let Some(hex) = inner.strip_prefix("#x").or_else(|| inner.strip_prefix("#X")) {
        return u32::from_str_radix(hex, 16).ok().and_then(char::from_u32);
    }
    if let Some(dec) = inner.strip_prefix('#') {
        return dec.parse::<u32>().ok().and_then(char::from_u32);
    }

    // Named entities
    match inner {
        "lt" => Some('<'),
        "gt" => Some('>'),
        "amp" => Some('&'),
        "quot" => Some('"'),
        "apos" => Some('\''),
        "nbsp" => Some('\u{00A0}'),
        _ => None,
    }
}

/// Check if a string starting with & is a valid HTML entity start
fn is_html_entity_start(s: &str) -> bool {
    let Some(rest) = s.strip_prefix('&') else {
        return false;
    };

    if rest.is_empty() {
        return false;
    }

    // Numeric entity: &#x... (hex) or &#... (decimal)
    if let Some(after_hash) = rest.strip_prefix('#') {
        // Hex: &#x followed by hex digits
        if let Some(hex_part) =
            after_hash.strip_prefix('x').or_else(|| after_hash.strip_prefix('X'))
        {
            return hex_part.chars().next().is_some_and(|c| c.is_ascii_hexdigit());
        }
        // Decimal: &# followed by digits
        return after_hash.chars().next().is_some_and(|c| c.is_ascii_digit());
    }

    // Named entity: & followed by alphanumeric, eventually ending with ;
    // Check if the next char is alphanumeric (common named entities like &quot;, &amp;, etc.)
    rest.chars().next().is_some_and(|c| c.is_ascii_alphanumeric())
}

/// Derive the component variable name from the filename.
///
/// Matches the Go compiler's `getComponentName` logic:
/// - No filename → `$$Component`
/// - Extract the file stem (before first `.`)
/// - Convert to PascalCase
/// - If the result is `Astro`, use `$$Component` (avoid collision)
/// - Otherwise use `$$` + PascalCase name
///
/// Examples:
///   `/src/pages/index.astro` → `$$Index`
///   `/src/pages/about-us.astro` → `$$AboutUs`
///   `/src/pages/page-with-'-quotes.astro` → `$$PageWithQuotes`
fn get_component_name(filename: Option<&str>) -> String {
    let Some(filename) = filename else {
        return "$$Component".to_string();
    };
    if filename.is_empty() {
        return "$$Component".to_string();
    }

    // Get the last path segment
    let part = filename.rsplit('/').next().unwrap_or("");
    if part.is_empty() {
        return "$$Component".to_string();
    }

    // Get the stem (before first `.`)
    let stem = part.split('.').next().unwrap_or(part);
    if stem.is_empty() {
        return "$$Component".to_string();
    }

    // Convert to PascalCase: split on non-alphanumeric, capitalize each word
    let pascal = stem
        .split(|c: char| !c.is_ascii_alphanumeric())
        .filter(|s| !s.is_empty())
        .map(|word| {
            let mut chars = word.chars();
            match chars.next() {
                Some(first) => {
                    let upper: String = first.to_uppercase().collect();
                    format!("{upper}{}", chars.as_str())
                }
                None => String::new(),
            }
        })
        .collect::<String>();

    if pascal.is_empty() || pascal == "Astro" {
        return "$$Component".to_string();
    }

    format!("$${pascal}")
}

/// Strip TypeScript syntax from generated code.
///
/// Parses the code as TypeScript, runs `oxc_transformer` (TS-only stripping,
/// no JSX transform, no ES downleveling), and re-emits as JavaScript.
fn strip_typescript(allocator: &Allocator, code: &str) -> String {
    let source_type = oxc_span::SourceType::mjs().with_typescript(true);
    let ret = oxc_parser::Parser::new(allocator, code, source_type).parse();

    if !ret.errors.is_empty() {
        // If parsing fails, return the code unchanged — the downstream
        // consumer will report a better error.
        return code.to_string();
    }

    let mut program = ret.program;
    let scoping = oxc_semantic::SemanticBuilder::new()
        .with_excess_capacity(2.0)
        .build(&program)
        .semantic
        .into_scoping();

    let mut options = oxc_transformer::TransformOptions::default();
    // Keep value imports that appear unused. In our generated code, imported
    // identifiers are referenced inside template literal strings (e.g.
    // `$$render\`${Component}\``) which semantic analysis cannot see, so
    // without this flag the transformer would incorrectly remove them.
    // This flag only affects import elision — all other TS stripping
    // (type annotations, interfaces, `as` casts, etc.) happens regardless.
    options.typescript.only_remove_type_imports = true;
    let _ = oxc_transformer::Transformer::new(allocator, std::path::Path::new(""), &options)
        .build_with_scoping(scoping, &mut program);

    let codegen_options = oxc_codegen::CodegenOptions {
        single_quote: false,
        ..oxc_codegen::CodegenOptions::default()
    };
    oxc_codegen::Codegen::new().with_options(codegen_options).build(&program).code
}

#[cfg(test)]
#[expect(clippy::needless_raw_string_hashes, clippy::uninlined_format_args, clippy::print_stderr)]
mod tests {
    use super::*;
    use oxc_allocator::Allocator;
    use oxc_parser::Parser;
    use oxc_span::SourceType;

    fn compile_astro(source: &str) -> String {
        compile_astro_with_options(source, TransformOptions::new().with_internal_url("http://localhost:3000/")).code
    }

    fn compile_astro_with_options(source: &str, options: TransformOptions) -> TransformResult {
        let allocator = Allocator::default();
        let source_type = SourceType::astro();
        let ret = Parser::new(&allocator, source, source_type).parse_astro();
        assert!(ret.errors.is_empty(), "Parse errors: {:?}", ret.errors);

        let codegen = AstroCodegen::new(&allocator, source, options);
        codegen.build(&ret.root)
    }

    #[test]
    fn test_basic_no_frontmatter() {
        let source = "<button>Click</button>";
        let output = compile_astro(source);

        assert!(output.contains("import {"));
        assert!(output.contains("$$createComponent"));
        assert!(output.contains("<button>Click</button>"));
        assert!(output.contains("export default $$Component"));
    }

    #[test]
    fn test_basic_with_frontmatter() {
        let source = r#"---
const href = '/about';
---
<a href={href}>About</a>"#;
        let output = compile_astro(source);

        // Note: codegen normalizes strings to double quotes
        assert!(output.contains("const href = \"/about\""), "Missing const declaration");
        assert!(output.contains("$$addAttribute(href, \"href\")"), "Missing $$addAttribute");
    }

    #[test]
    fn test_component_rendering() {
        let source = r#"---
import Component from 'test';
---
<Component />"#;
        let output = compile_astro(source);

        // Note: codegen normalizes strings to double quotes
        assert!(output.contains("import Component from \"test\""), "Missing import");
        assert!(output.contains("$$renderComponent"), "Missing $$renderComponent");
        assert!(output.contains("\"Component\""), "Missing component name");
    }

    #[test]
    fn test_doctype() {
        let source = "<!DOCTYPE html><div></div>";
        let output = compile_astro(source);

        // Doctype is stripped in output, only the div appears in the template
        assert!(output.contains("<div></div>"), "Missing div element");
        assert!(output.contains("$$maybeRenderHead"), "Missing maybeRenderHead");
    }

    #[test]
    fn test_fragment() {
        let source = "<><div>1</div><div>2</div></>";
        let output = compile_astro(source);

        assert!(output.contains("$$renderComponent"), "Missing renderComponent");
        assert!(output.contains("Fragment"), "Missing Fragment reference");
        assert!(output.contains("<div>1</div>"), "Missing first div");
        assert!(output.contains("<div>2</div>"), "Missing second div");
    }

    #[test]
    fn test_html_head_body() {
        let source = r#"<html>
  <head>
    <title>Test</title>
  </head>
  <body>
    <h1>Hello</h1>
  </body>
</html>"#;
        let output = compile_astro(source);

        // Should have renderHead in head, not maybeRenderHead
        assert!(output.contains("$$renderHead($$result)"), "Missing renderHead in head");
        assert!(output.contains("<title>Test</title>"), "Missing title");
        assert!(output.contains("<h1>Hello</h1>"), "Missing h1");
    }

    #[test]
    fn test_expression_in_attribute() {
        let source = r#"---
const src = "image.png";
---
<img src={src} alt="test" />"#;
        let output = compile_astro(source);

        assert!(output.contains("$$addAttribute(src, \"src\")"), "Missing dynamic src attribute");
        assert!(output.contains("alt=\"test\""), "Missing static alt attribute");
    }

    #[test]
    fn test_expression_in_content() {
        let source = r#"---
const name = "World";
---
<h1>Hello {name}!</h1>"#;
        let output = compile_astro(source);

        assert!(output.contains("Hello ${name}!"), "Missing interpolated expression");
    }

    #[test]
    fn test_slots_basic() {
        let source = r#"---
import Component from "test";
---
<Component>
    <div>Default</div>
    <div slot="named">Named</div>
</Component>"#;
        let output = compile_astro(source);

        assert!(output.contains("\"default\":"), "Missing default slot");
        assert!(output.contains("\"named\":"), "Missing named slot");
    }

    #[test]
    fn test_conditional_slot() {
        // Expression containing a slotted element should extract the slot name
        let source = r#"---
import Component from "test";
---
<Component>{value && <div slot="test">foo</div>}</Component>"#;
        let output = compile_astro(source);

        // Should have "test" slot, not "default"
        assert!(output.contains("\"test\":"), "Missing named slot 'test'");
        // The slot attribute should be removed from the element
        assert!(!output.contains("slot=\"test\""), "Slot attribute should be removed from element");
        assert!(output.contains("<div>foo</div>"), "Missing div element without slot attr");
    }

    #[test]
    fn test_expression_slot_multiple() {
        // Multiple expressions with different slots
        let source = r#"---
import Component from "test";
---
<Component>{true && <div slot="a">A</div>}{false && <div slot="b">B</div>}</Component>"#;
        let output = compile_astro(source);

        // Should have both named slots
        assert!(output.contains("\"a\":"), "Missing named slot 'a'");
        assert!(output.contains("\"b\":"), "Missing named slot 'b'");
        // Neither should be default
        assert!(!output.contains("\"default\":"), "Should not have default slot");
    }

    #[test]
    fn test_client_load_directive() {
        let source = r#"---
import Component from 'test';
---
<Component client:load />"#;
        let output = compile_astro(source);

        eprintln!("OUTPUT:\n{output}");
        assert!(
            output.contains("client:component-hydration") && output.contains("load"),
            "Missing hydration directive"
        );
    }

    #[test]
    fn test_void_elements() {
        // Void elements should NOT have closing tags
        let source = r#"<meta charset="utf-8"><input type="text"><br><img src="x.png"><link rel="stylesheet" href="style.css"><hr>"#;
        let output = compile_astro(source);

        // Should contain opening tags
        assert!(output.contains("<meta charset=\"utf-8\">"), "Missing meta tag");
        assert!(output.contains("<input type=\"text\">"), "Missing input tag");
        assert!(output.contains("<br>"), "Missing br tag");
        assert!(output.contains("<img"), "Missing img tag");
        assert!(output.contains("<link"), "Missing link tag");
        assert!(output.contains("<hr>"), "Missing hr tag");

        // Should NOT contain closing tags for void elements
        assert!(
            !output.contains("</meta>"),
            "Found </meta> - void elements should not have closing tags"
        );
        assert!(
            !output.contains("</input>"),
            "Found </input> - void elements should not have closing tags"
        );
        assert!(
            !output.contains("</br>"),
            "Found </br> - void elements should not have closing tags"
        );
        assert!(
            !output.contains("</img>"),
            "Found </img> - void elements should not have closing tags"
        );
        assert!(
            !output.contains("</link>"),
            "Found </link> - void elements should not have closing tags"
        );
        assert!(
            !output.contains("</hr>"),
            "Found </hr> - void elements should not have closing tags"
        );
    }

    #[test]
    fn test_no_maybe_render_head_with_explicit_head() {
        // When there's an explicit <head> element, $$renderHead is used inside head
        // and $$maybeRenderHead should NOT be inserted in the body
        let source = r#"<html>
  <head>
    <title>Test</title>
  </head>
  <body>
    <main>
      <h1>Hello</h1>
    </main>
  </body>
</html>"#;
        let output = compile_astro(source);

        // Should have $$renderHead inside head (before closing tag, possibly with whitespace)
        assert!(output.contains("$$renderHead($$result)"), "Missing $$renderHead in head");

        // Should NOT have $$maybeRenderHead in the body
        // Count occurrences of $$maybeRenderHead - should only appear in import
        let maybe_render_head_count = output.matches("$$maybeRenderHead").count();
        assert_eq!(
            maybe_render_head_count, 1,
            "$$maybeRenderHead should only appear once (in import), found {} times. Body should not have $$maybeRenderHead when explicit <head> exists",
            maybe_render_head_count
        );
    }

    #[test]
    fn test_head_elements_skip_maybe_render_head() {
        // Head-related elements (link, meta, script, style, title, etc.) should NOT
        // trigger $$maybeRenderHead insertion. This matches Go compiler behavior.
        let source = r#"<Component /><link href="style.css"><meta charset="utf-8"><script src="app.js"></script>"#;
        let output = compile_astro(source);

        // Should have the elements rendered
        assert!(output.contains("<link href=\"style.css\">"), "Missing link element");
        assert!(output.contains("<meta charset=\"utf-8\">"), "Missing meta element");

        // Should NOT have $$maybeRenderHead before these head elements
        // Only the import should have $$maybeRenderHead
        let maybe_render_head_count = output.matches("$$maybeRenderHead").count();
        assert_eq!(
            maybe_render_head_count, 1,
            "$$maybeRenderHead should only appear once (in import), found {} times. Head elements should not trigger $$maybeRenderHead",
            maybe_render_head_count
        );
    }

    #[test]
    fn test_custom_element() {
        // Custom elements (with dashes in name) should be rendered as components
        // using $$renderComponent with the tag name as a quoted string
        let source = r#"<my-element foo="bar"></my-element>"#;
        let output = compile_astro(source);

        // Should use $$renderComponent, not render as HTML
        assert!(
            output.contains("$$renderComponent"),
            "Custom elements should use $$renderComponent"
        );
        assert!(
            output.contains("\"my-element\"") && output.matches("\"my-element\"").count() >= 2,
            "Custom element should have tag name as both display name and quoted identifier"
        );

        // Should NOT trigger $$maybeRenderHead (custom elements are like components)
        let maybe_render_head_count = output.matches("$$maybeRenderHead").count();
        assert_eq!(
            maybe_render_head_count, 1,
            "$$maybeRenderHead should only appear once (in import), custom elements should not trigger it"
        );

        // Should NOT be rendered as HTML
        assert!(
            !output.contains("<my-element"),
            "Custom elements should not be rendered as HTML tags"
        );
    }

    #[test]
    fn test_html_comments_preserved() {
        // HTML comments should be preserved in the output
        let source = r#"<!-- Global Metadata -->
<meta charset="utf-8">
<!-- Another comment -->
<link rel="icon" href="/favicon.ico" />"#;
        let output = compile_astro(source);

        eprintln!("Output: {}", output);

        // HTML comments should be preserved
        assert!(output.contains("<!-- Global Metadata -->"), "Missing first HTML comment");
        assert!(output.contains("<!-- Another comment -->"), "Missing second HTML comment");
        // Elements should also be present
        assert!(output.contains("<meta charset=\"utf-8\">"), "Missing meta tag");
        assert!(output.contains("<link rel=\"icon\" href=\"/favicon.ico\">"), "Missing link tag");
    }

    // === Metadata tests ===

    #[test]
    fn test_hydrated_component_metadata_default_import() {
        let source = r#"---
import One from "../components/one.jsx";
---
<One client:load />"#;
        let result = compile_astro_with_options(
            source,
            TransformOptions::new().with_internal_url("http://localhost:3000/"),
        );

        assert_eq!(result.hydrated_components.len(), 1);
        let c = &result.hydrated_components[0];
        assert_eq!(c.export_name, "default");
        assert_eq!(c.local_name, "One");
        assert_eq!(c.specifier, "../components/one.jsx");
    }

    #[test]
    fn test_hydrated_component_metadata_named_import() {
        let source = r#"---
import { Three } from "../components/three.tsx";
---
<Three client:load />"#;
        let result = compile_astro_with_options(
            source,
            TransformOptions::new().with_internal_url("http://localhost:3000/"),
        );

        assert_eq!(result.hydrated_components.len(), 1);
        let c = &result.hydrated_components[0];
        assert_eq!(c.export_name, "Three");
        assert_eq!(c.local_name, "Three");
        assert_eq!(c.specifier, "../components/three.tsx");
    }

    #[test]
    fn test_hydrated_component_metadata_namespace_dot_notation() {
        let source = r#"---
import * as Two from "../components/two.jsx";
---
<Two.someName client:load />"#;
        let result = compile_astro_with_options(
            source,
            TransformOptions::new().with_internal_url("http://localhost:3000/"),
        );

        assert_eq!(result.hydrated_components.len(), 1);
        let c = &result.hydrated_components[0];
        assert_eq!(c.export_name, "someName");
        assert_eq!(c.local_name, "Two.someName");
        assert_eq!(c.specifier, "../components/two.jsx");
    }

    #[test]
    fn test_hydrated_component_metadata_namespace_deep_dot_notation() {
        let source = r#"---
import * as four from "../components/four.jsx";
---
<four.nested.deep.Component client:load />"#;
        let result = compile_astro_with_options(
            source,
            TransformOptions::new().with_internal_url("http://localhost:3000/"),
        );

        assert_eq!(result.hydrated_components.len(), 1);
        let c = &result.hydrated_components[0];
        assert_eq!(c.export_name, "nested.deep.Component");
        assert_eq!(c.local_name, "four.nested.deep.Component");
        assert_eq!(c.specifier, "../components/four.jsx");
    }

    #[test]
    fn test_client_only_component_metadata() {
        let source = r#"---
import Five from "../components/five.jsx";
---
<Five client:only="react" />"#;
        let result = compile_astro_with_options(
            source,
            TransformOptions::new().with_internal_url("http://localhost:3000/"),
        );

        assert_eq!(result.client_only_components.len(), 1);
        let c = &result.client_only_components[0];
        assert_eq!(c.export_name, "default");
        assert_eq!(c.local_name, "Five");
        assert_eq!(c.specifier, "../components/five.jsx");
    }

    #[test]
    fn test_client_only_component_metadata_named() {
        let source = r#"---
import { Named } from "../components/named.jsx";
---
<Named client:only="react" />"#;
        let result = compile_astro_with_options(
            source,
            TransformOptions::new().with_internal_url("http://localhost:3000/"),
        );

        assert_eq!(result.client_only_components.len(), 1);
        let c = &result.client_only_components[0];
        assert_eq!(c.export_name, "Named");
        assert_eq!(c.local_name, "Named");
        assert_eq!(c.specifier, "../components/named.jsx");
    }

    #[test]
    fn test_client_only_component_metadata_star_export() {
        let source = r#"---
import * as Five from "../components/five.jsx";
---
<Five.someName client:only />"#;
        let result = compile_astro_with_options(
            source,
            TransformOptions::new()
                .with_filename("/users/astro/apps/pacman/src/pages/index.astro"),
        );

        assert_eq!(result.client_only_components.len(), 1);
        let c = &result.client_only_components[0];
        assert_eq!(c.export_name, "someName");
        assert_eq!(c.specifier, "../components/five.jsx");
    }

    #[test]
    fn test_client_only_component_metadata_deep_nested() {
        let source = r#"---
import * as eight from "../components/eight.jsx";
---
<eight.nested.deep.Component client:only />"#;
        let result = compile_astro_with_options(
            source,
            TransformOptions::new()
                .with_filename("/users/astro/apps/pacman/src/pages/index.astro"),
        );

        assert_eq!(result.client_only_components.len(), 1);
        let c = &result.client_only_components[0];
        assert_eq!(c.export_name, "nested.deep.Component");
        assert_eq!(c.specifier, "../components/eight.jsx");
    }

    #[test]
    fn test_server_deferred_component_metadata() {
        let source = r#"---
import Avatar from "../components/Avatar.jsx";
import { Other } from "../components/Other.jsx";
---
<Avatar server:defer />
<Other server:defer />"#;
        let result = compile_astro_with_options(
            source,
            TransformOptions::new().with_internal_url("http://localhost:3000/"),
        );

        assert_eq!(result.server_components.len(), 2);

        let c0 = &result.server_components[0];
        assert_eq!(c0.export_name, "default");
        assert_eq!(c0.local_name, "Avatar");
        assert_eq!(c0.specifier, "../components/Avatar.jsx");

        let c1 = &result.server_components[1];
        assert_eq!(c1.export_name, "Other");
        assert_eq!(c1.local_name, "Other");
        assert_eq!(c1.specifier, "../components/Other.jsx");

        // server:defer should also set propagation = true
        assert!(result.propagation, "server:defer should enable propagation");
    }

    #[test]
    fn test_contains_head_metadata() {
        let source = r#"<html>
<head><title>Test</title></head>
<body><p>content</p></body>
</html>"#;
        let result = compile_astro_with_options(
            source,
            TransformOptions::new().with_internal_url("http://localhost:3000/"),
        );

        assert!(result.contains_head, "Should detect explicit <head>");
    }

    #[test]
    fn test_no_head_metadata() {
        let source = "<p>no head here</p>";
        let result = compile_astro_with_options(
            source,
            TransformOptions::new().with_internal_url("http://localhost:3000/"),
        );

        assert!(!result.contains_head, "Should not detect <head> when absent");
    }

    #[test]
    fn test_resolve_path_filepath_join_fallback() {
        let source = r#"---
import Counter from "../components/Counter.jsx";
---
<Counter client:load />"#;
        let result = compile_astro_with_options(
            source,
            TransformOptions::new()
                .with_internal_url("http://localhost:3000/")
                .with_filename("src/pages/index.astro"),
        );

        // Without resolvePath, fallback uses filepath.Join
        assert_eq!(result.hydrated_components.len(), 1);
        let c = &result.hydrated_components[0];
        assert_eq!(c.specifier, "../components/Counter.jsx");
        // resolved_path should be filepath.Join("src/pages", "../components/Counter.jsx")
        assert_eq!(c.resolved_path, "src/components/Counter.jsx");
    }

    #[test]
    fn test_resolve_path_custom_function() {
        let source = r#"---
import Counter from "../components/Counter.jsx";
---
<Counter client:load />"#;
        let result = compile_astro_with_options(
            source,
            TransformOptions::new()
                .with_internal_url("http://localhost:3000/")
                .with_filename("src/pages/index.astro")
                .with_resolve_path(|specifier| format!("/resolved{specifier}")),
        );

        assert_eq!(result.hydrated_components.len(), 1);
        let c = &result.hydrated_components[0];
        assert_eq!(c.resolved_path, "/resolved../components/Counter.jsx");

        // Code should NOT contain $$createMetadata when resolvePath is provided
        assert!(!result.code.contains("$$createMetadata"), "Should skip $$createMetadata");
        assert!(!result.code.contains("$$metadata"), "Should skip $$metadata export");
        assert!(!result.code.contains("$$module1"), "Should skip $$module imports");
    }

    #[test]
    fn test_resolve_path_bare_specifier_fallback() {
        let source = r#"---
import Counter from "some-package";
---
<Counter client:load />"#;
        let result = compile_astro_with_options(
            source,
            TransformOptions::new()
                .with_internal_url("http://localhost:3000/")
                .with_filename("src/pages/index.astro"),
        );

        // Bare specifier (doesn't start with '.') should be returned as-is
        assert_eq!(result.hydrated_components.len(), 1);
        assert_eq!(result.hydrated_components[0].resolved_path, "some-package");
    }

    #[test]
    fn test_server_defer_skips_attribute() {
        let source = r#"---
import Avatar from "./Avatar.jsx";
---
<Avatar server:defer />"#;
        let result = compile_astro_with_options(
            source,
            TransformOptions::new().with_internal_url("http://localhost:3000/"),
        );

        // server:defer should not appear as a rendered prop
        assert!(!result.code.contains("\"server:defer\""), "server:defer should be stripped from props");
    }

    #[test]
    fn test_typescript_satisfies_stripped() {
        let source = r#"---
interface SEOProps { title: string; }
const seo = { title: 'Hello' } satisfies SEOProps;
---
<h1>{seo.title}</h1>"#;
        let output = compile_astro(source);

        // `satisfies` is TypeScript-only syntax — must be stripped
        assert!(!output.contains("satisfies"), "satisfies keyword should be stripped: {}", output);
        // The interface should also be stripped
        assert!(!output.contains("interface SEOProps"), "interface should be stripped: {}", output);
        // The value expression should remain
        assert!(output.contains("title: \"Hello\"") || output.contains("title: 'Hello'"),
            "value expression should remain: {}", output);
    }

    #[test]
    fn test_type_only_import_stripped() {
        let source = r#"---
import type { Props } from './types';
const x: Props = { title: 'hi' };
---
<h1>{x.title}</h1>"#;
        let output = compile_astro(source);

        // `import type` should be stripped by strip_typescript
        assert!(!output.contains("import type"), "import type should be stripped: {}", output);
    }
}
