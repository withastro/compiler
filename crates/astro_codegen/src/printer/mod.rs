//! Astro code printer.
//!
//! Transforms an `AstroRoot` AST into JavaScript code compatible with the Astro runtime.
//!
//! This module is split into focused submodules:
//!
//! - [`escape`] — string escaping and HTML entity decoding utilities
//! - [`result`] — public output types (`TransformResult`, etc.)
//! - [`slots`] — slot analysis and slot printing
//! - [`expressions`] — JS/JSX expression printing
//! - [`components`] — component element printing and hydration
//! - [`elements`] — HTML element printing, attributes, and element utilities

use oxc_allocator::Allocator;
use oxc_ast::ast::*;
use oxc_data_structures::code_buffer::CodeBuffer;
use rustc_hash::FxHashMap;

use oxc_codegen::{Codegen, Context, Gen};

use crate::TransformOptions;
use crate::scanner::{
    AstroScanner, HoistedScriptType as InternalHoistedScriptType, ScanResult,
    get_jsx_attribute_name, get_jsx_element_name, is_component_name, is_custom_element,
    should_hoist_script,
};

mod components;
mod elements;
mod escape;
mod expressions;
pub mod result;
mod slots;

// Re-export public result types at the `printer` level so that `lib.rs`
// can `pub use printer::{...}` without reaching into `result`.
pub use result::{
    HoistedScriptType, TransformResult, TransformResultHoistedScript,
    TransformResultHydratedComponent,
};

// Bring escape helpers into scope for use inside this file.
use escape::{escape_single_quote, escape_template_literal};

// Bring element helpers into scope for use inside this file.
use elements::is_head_element;

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

/// Transform an Astro AST into JavaScript code.
///
/// This is the primary entry point for code generation. It runs the scanner
/// (analysis pass) and printer (code generation pass) in sequence, then strips
/// TypeScript syntax from the output.
///
/// ```ignore
/// let allocator = Allocator::default();
/// let ret = Parser::new(&allocator, source, SourceType::astro()).parse_astro();
/// let result = transform(&allocator, source, options, &ret.root);
/// println!("{}", result.code);
/// ```
pub fn transform<'a>(
    allocator: &'a Allocator,
    source_text: &'a str,
    options: TransformOptions,
    root: &'a AstroRoot<'a>,
) -> TransformResult {
    let scan_result = AstroScanner::new(allocator).scan(root);
    let codegen = AstroCodegen::new(allocator, source_text, options, scan_result);
    codegen.build(root)
}

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
    /// Result from the scanner pass
    scan_result: ScanResult,
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
    transition_counter: usize,
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
    /// The import specifier (e.g., `"../components"`)
    specifier: String,
    /// The export name (`"default"` for default imports, otherwise the named export)
    export_name: String,
    /// Whether this is a namespace import (`import * as x`)
    is_namespace: bool,
}

impl<'a> AstroCodegen<'a> {
    /// Create a new Astro codegen instance.
    ///
    /// The `scan_result` must be obtained by running [`AstroScanner`] on the
    /// same `AstroRoot` that will be passed to [`build`](Self::build).
    pub fn new(
        allocator: &'a Allocator,
        source_text: &'a str,
        options: TransformOptions,
        scan_result: ScanResult,
    ) -> Self {
        // Compute base hash for the source file
        let source_hash = Self::compute_source_hash(source_text);

        Self {
            allocator,
            options,
            code: CodeBuffer::default(),
            source_text,
            scan_result,
            in_head: false,
            render_head_inserted: false,
            has_explicit_head: false,
            module_imports: Vec::new(),
            component_imports: FxHashMap::default(),
            skip_slot_attribute: false,
            script_index: 0,
            element_depth: 0,
            transition_counter: 0,
            source_hash,
        }
    }

    /// Compute a hash of the source text (similar to Go's xxhash + base32).
    fn compute_source_hash(source: &str) -> String {
        use std::collections::hash_map::DefaultHasher;
        use std::hash::{Hash, Hasher};

        let mut hasher = DefaultHasher::new();
        source.hash(&mut hasher);
        let hash = hasher.finish();
        Self::to_base32_like(hash)
    }

    /// Convert a u64 hash to a lowercase alphanumeric string (similar to base32).
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

    /// Resolve a component name (possibly dot-notation like `"Two.someName"`) to
    /// its metadata, looking up the root import name in `component_imports`.
    fn resolve_component_metadata(
        &self,
        component_name: &str,
    ) -> Option<TransformResultHydratedComponent> {
        if component_name.contains('.') {
            let dot_pos = component_name.find('.').unwrap();
            let root = &component_name[..dot_pos];
            let rest = &component_name[dot_pos + 1..];

            let info = self.component_imports.get(root)?;
            let export_name = if info.is_namespace {
                rest.to_string()
            } else if info.export_name == "default" {
                format!("default.{rest}")
            } else {
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
    fn needs_async(&self) -> bool {
        self.scan_result.has_await
    }

    /// Get the async function prefix if needed (`"async "` or `""`).
    fn get_async_prefix(&self) -> &'static str {
        if self.needs_async() { "async " } else { "" }
    }

    /// Get the slot callback parameter list.
    fn get_slot_params(&self) -> &'static str {
        if self.options.result_scoped_slot {
            "($$result) => "
        } else {
            "() => "
        }
    }

    // --- Output helpers ---

    fn print(&mut self, s: &str) {
        self.code.print_str(s);
    }

    fn println(&mut self, s: &str) {
        self.code.print_str(s);
        self.code.print_char('\n');
    }

    // --- Build ---

    /// Build the JavaScript output from an Astro AST.
    pub fn build(mut self, root: &'a AstroRoot<'a>) -> TransformResult {
        self.print_astro_root(root);

        // Build public hoisted scripts from internal representation
        let scripts = self
            .scan_result
            .hoisted_scripts
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
            .scan_result
            .hydrated_components
            .iter()
            .filter_map(|h| self.resolve_component_metadata(&h.name))
            .collect();

        // Build public client-only components from scanner's full names
        let client_only_components = self
            .scan_result
            .client_only_components
            .iter()
            .filter_map(|h| self.resolve_component_metadata(&h.name))
            .collect();

        // Build public server components from internal representation
        let server_components = self
            .scan_result
            .server_deferred_components
            .iter()
            .filter_map(|h| {
                let mut meta = self.resolve_component_metadata(&h.name)?;
                meta.local_name.clone_from(&h.name);
                Some(meta)
            })
            .collect();

        let propagation = self.scan_result.uses_transitions;
        let contains_head = self.has_explicit_head;
        let scope = self.source_hash.clone();
        let code = self.code.into_string();

        // Strip TypeScript from the generated code.
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

    // --- Orchestration / top-level printing ---

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

        // Blank line after user imports
        if !imports.is_empty() {
            self.println("");
        }

        // 3. Print namespace imports for modules (for metadata) - skip client:only components
        self.print_namespace_imports();

        // 4. Print metadata export
        if imports.is_empty() {
            self.println("");
        }
        self.print_metadata();

        // 5. Print top-level Astro global if needed
        if self.scan_result.uses_astro_global {
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

    fn print_internal_imports(&mut self) {
        let url = self.options.get_internal_url().to_string();

        self.println("import {");
        self.println(&format!("  {},", runtime::FRAGMENT));
        self.println(&format!("  render as {},", runtime::RENDER));
        self.println(&format!("  createAstro as {},", runtime::CREATE_ASTRO));
        self.println(&format!(
            "  createComponent as {},",
            runtime::CREATE_COMPONENT
        ));
        self.println(&format!(
            "  renderComponent as {},",
            runtime::RENDER_COMPONENT
        ));
        self.println(&format!("  renderHead as {},", runtime::RENDER_HEAD));
        self.println(&format!(
            "  maybeRenderHead as {},",
            runtime::MAYBE_RENDER_HEAD
        ));
        self.println(&format!("  unescapeHTML as {},", runtime::UNESCAPE_HTML));
        self.println(&format!("  renderSlot as {},", runtime::RENDER_SLOT));
        self.println(&format!("  mergeSlots as {},", runtime::MERGE_SLOTS));
        self.println(&format!("  addAttribute as {},", runtime::ADD_ATTRIBUTE));
        self.println(&format!(
            "  spreadAttributes as {},",
            runtime::SPREAD_ATTRIBUTES
        ));
        self.println(&format!(
            "  defineStyleVars as {},",
            runtime::DEFINE_STYLE_VARS
        ));
        self.println(&format!(
            "  defineScriptVars as {},",
            runtime::DEFINE_SCRIPT_VARS
        ));
        self.println(&format!(
            "  renderTransition as {},",
            runtime::RENDER_TRANSITION
        ));
        self.println(&format!(
            "  createTransitionScope as {},",
            runtime::CREATE_TRANSITION_SCOPE
        ));
        self.println(&format!("  renderScript as {},", runtime::RENDER_SCRIPT));
        if !self.options.has_resolve_path() {
            self.println(&format!("  createMetadata as {}", runtime::CREATE_METADATA));
        }
        self.println(&format!("}} from \"{url}\";"));

        if self.scan_result.uses_transitions {
            self.println("import \"transitions.css\";");
        }
    }

    fn print_namespace_imports(&mut self) {
        if self.module_imports.is_empty() || self.options.has_resolve_path() {
            return;
        }

        let imports: Vec<_> = self.module_imports.clone();
        for import in imports {
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
        let hydrated_str = if self.scan_result.hydrated_components.is_empty() {
            "[]".to_string()
        } else {
            let custom_elements: Vec<String> = self
                .scan_result
                .hydrated_components
                .iter()
                .filter(|c| c.is_custom_element)
                .map(|c| format!("\"{}\"", c.name))
                .collect();

            let regular_components: Vec<String> = self
                .scan_result
                .hydrated_components
                .iter()
                .filter(|c| !c.is_custom_element)
                .rev()
                .map(|c| c.name.clone())
                .collect();

            let mut items = custom_elements;
            items.extend(regular_components);
            format!("[{}]", items.join(","))
        };

        // Build client-only components array
        let client_only_str = {
            let scan_client_only = &self.scan_result.client_only_components;
            if scan_client_only.is_empty() {
                "[]".to_string()
            } else {
                let mut seen = Vec::new();
                let mut items = Vec::new();
                for h in scan_client_only {
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
        let directives_str = if self.scan_result.hydration_directives.is_empty() {
            "new Set([])".to_string()
        } else {
            let items: Vec<String> = self
                .scan_result
                .hydration_directives
                .iter()
                .map(|s| format!("\"{s}\""))
                .collect();
            format!("new Set([{}])", items.join(", "))
        };

        // Build hoisted scripts array
        let hoisted_str = if self.scan_result.hoisted_scripts.is_empty() {
            "[]".to_string()
        } else {
            let items: Vec<String> = self
                .scan_result.hoisted_scripts
                .iter()
                .map(|script| {
                    match script.script_type {
                        InternalHoistedScriptType::Inline => {
                            let value = script.value.as_deref().unwrap_or("");
                            let escaped = escape_template_literal(value);
                            format!("{{ type: \"inline\", value: `{escaped}` }}")
                        }
                        InternalHoistedScriptType::External => {
                            let src = script.src.as_deref().unwrap_or("");
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
        let astro_global_args = self
            .options
            .astro_global_args
            .as_deref()
            .unwrap_or("\"https://astro.build\"");

        self.println(&format!(
            "const $$Astro = {}({});",
            runtime::CREATE_ASTRO,
            astro_global_args
        ));
        self.println("const Astro = $$Astro;");
    }

    /// Split frontmatter into three categories:
    /// - imports: import declarations (hoisted to top of module)
    /// - exports: export declarations (hoisted after metadata, before component)
    /// - other: regular statements (inside component function)
    fn split_frontmatter<'b>(
        &mut self,
        frontmatter: Option<&'b AstroFrontmatter<'a>>,
    ) -> (
        Vec<&'b Statement<'a>>,
        Vec<&'b Statement<'a>>,
        Vec<&'b Statement<'a>>,
    )
    where
        'a: 'b,
    {
        let mut imports = Vec::new();
        let mut exports = Vec::new();
        let mut other = Vec::new();

        if let Some(fm) = frontmatter {
            let mut module_counter = 1;

            for stmt in &fm.program.body {
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
                    let source = import.source.value.as_str();

                    if import.import_kind == ImportOrExportKind::Type {
                        imports.push(stmt);
                    } else {
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
                                self.scan_result
                                    .client_only_component_names
                                    .contains(local_name)
                            })
                        } else {
                            false
                        };

                        let is_bare_css_import =
                            import.specifiers.is_none() && is_css_specifier(source);

                        if is_client_only_import {
                            // Client:only component imports are not needed at runtime
                        } else if is_bare_css_import {
                            imports.push(stmt);
                        } else {
                            imports.push(stmt);
                            let namespace_var = format!("$$module{module_counter}");

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
                    }
                } else {
                    other.push(stmt);
                }
            }
        }

        (imports, exports, other)
    }

    fn print_statement(&mut self, stmt: &Statement<'_>) {
        let mut codegen = Codegen::new();
        stmt.print(&mut codegen, Context::default().with_typescript());
        let code = codegen.into_source_text();
        let code = code.trim_end_matches('\n');
        self.println(code);
    }

    fn print_component_wrapper(
        &mut self,
        statements: &[&'a Statement<'a>],
        body: &[JSXChild<'a>],
        component_name: &str,
    ) {
        let async_prefix = self.get_async_prefix();
        self.println(&format!(
            "const {} = {}({}({}, $$props, $$slots) => {{",
            component_name,
            runtime::CREATE_COMPONENT,
            async_prefix,
            runtime::RESULT
        ));

        if self.scan_result.uses_astro_global {
            self.println(&format!(
                "const Astro = {}.createAstro($$props, $$slots);",
                runtime::RESULT
            ));
            self.println(&format!("Astro.self = {component_name};"));
        }

        self.println("");

        for stmt in statements {
            self.print_statement(stmt);
        }

        if !statements.is_empty() {
            self.println("");
        }

        self.print("return ");
        self.print(runtime::RENDER);
        self.print("`");

        if self.needs_maybe_render_head_at_start(body) {
            self.print(&format!(
                "${{{}({})}}",
                runtime::MAYBE_RENDER_HEAD,
                runtime::RESULT
            ));
            self.render_head_inserted = true;
        }

        self.print_jsx_children_skip_leading_whitespace(body);

        self.println("`;");

        let filename_part = match &self.options.filename {
            Some(f) => format!("'{}'", escape_single_quote(f)),
            None => "undefined".to_string(),
        };
        let propagation = if self.scan_result.uses_transitions {
            "\"self\""
        } else {
            "undefined"
        };
        self.println(&format!("}}, {filename_part}, {propagation});"));
    }

    // --- JSX dispatch ---

    /// Print JSX children, skipping leading whitespace-only text nodes.
    fn print_jsx_children_skip_leading_whitespace(&mut self, children: &[JSXChild<'a>]) {
        let mut started = false;
        for child in children {
            if !started {
                if let JSXChild::Text(text) = child
                    && text.value.trim().is_empty()
                {
                    continue;
                }
                if matches!(child, JSXChild::AstroDoctype(_)) {
                    continue;
                }
                started = true;
            }
            self.print_jsx_child(child);
        }
    }

    /// Check if we need to insert `$$maybeRenderHead` at the start of the template.
    fn needs_maybe_render_head_at_start(&self, body: &[JSXChild<'a>]) -> bool {
        if self.render_head_inserted || self.has_explicit_head {
            return false;
        }

        for child in body {
            match child {
                JSXChild::Text(text) => {
                    if text.value.trim().is_empty() {
                        continue;
                    }
                    return false;
                }
                JSXChild::Element(el) => {
                    let name = get_jsx_element_name(&el.opening_element.name);
                    if name == "html" {
                        return false;
                    }
                    if name == "slot" {
                        return false;
                    }
                    if name == "script"
                        && el
                            .children
                            .iter()
                            .any(|c| matches!(c, JSXChild::AstroScript(_)))
                    {
                        continue;
                    }
                    return !is_component_name(&name)
                        && !is_custom_element(&name)
                        && !is_head_element(&name);
                }
                JSXChild::Fragment(_) | JSXChild::ExpressionContainer(_) => {
                    return false;
                }
                _ => {}
            }
        }
        false
    }

    /// Check if this element needs `$$maybeRenderHead` inserted before it.
    fn needs_render_head(&self, name: &str) -> bool {
        if self.render_head_inserted || self.in_head {
            return false;
        }
        if is_component_name(name) {
            return false;
        }
        if is_custom_element(name) {
            return false;
        }
        if is_head_element(name) {
            return false;
        }
        if name == "body" && self.has_explicit_head {
            return false;
        }
        true
    }

    /// Insert `$$maybeRenderHead` if needed before an HTML element.
    fn maybe_insert_render_head(&mut self, name: &str) {
        if self.needs_render_head(name) {
            self.print(&format!(
                "${{{}({})}}",
                runtime::MAYBE_RENDER_HEAD,
                runtime::RESULT
            ));
            self.render_head_inserted = true;
        }
    }

    /// Dispatch printing a single JSX child node.
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
                // AstroScript is handled specially — already parsed TypeScript
            }
            JSXChild::AstroDoctype(_doctype) => {
                // Doctype is typically stripped in the output
            }
            JSXChild::AstroComment(comment) => {
                self.print_astro_comment(comment);
            }
        }
    }

    fn print_astro_comment(&mut self, comment: &AstroComment<'a>) {
        self.print("<!--");
        self.print(&escape_template_literal(comment.value.as_str()));
        self.print("-->");
    }

    fn print_jsx_text(&mut self, text: &JSXText<'a>) {
        let escaped = escape_template_literal(text.value.as_str());
        self.print(&escaped);
    }

    /// Dispatch a JSX element to either component or HTML element printing.
    fn print_jsx_element(&mut self, el: &JSXElement<'a>) {
        let name = get_jsx_element_name(&el.opening_element.name);

        // Handle <script> elements that should be hoisted
        if name == "script" && Self::is_hoisted_script(el) {
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
            return;
        }

        let is_component = is_component_name(&name);
        let is_custom = is_custom_element(&name);

        self.element_depth += 1;

        if is_component || is_custom {
            self.print_component_element(el, &name);
        } else {
            self.print_html_element(el, &name);
        }

        self.element_depth -= 1;
    }

    /// Check if a script element should be hoisted.
    fn is_hoisted_script(el: &JSXElement<'a>) -> bool {
        if !should_hoist_script(&el.opening_element.attributes) {
            return false;
        }

        if el
            .children
            .iter()
            .any(|child| matches!(child, JSXChild::AstroScript(_)))
        {
            return true;
        }

        let has_text_content = el.children.iter().any(|child| {
            if let JSXChild::Text(text) = child {
                !text.value.trim().is_empty()
            } else {
                false
            }
        });

        let has_src = el.opening_element.attributes.iter().any(|attr| {
            if let JSXAttributeItem::Attribute(attr) = attr {
                get_jsx_attribute_name(&attr.name) == "src"
            } else {
                false
            }
        });

        has_text_content || has_src
    }
}

/// Check if an import specifier refers to a CSS file.
fn is_css_specifier(specifier: &str) -> bool {
    matches!(
        specifier.rsplit('.').next(),
        Some("css" | "pcss" | "postcss" | "sass" | "scss" | "styl" | "stylus" | "less")
    )
}

/// Derive the component variable name from the filename.
fn get_component_name(filename: Option<&str>) -> String {
    let Some(filename) = filename else {
        return "$$Component".to_string();
    };
    if filename.is_empty() {
        return "$$Component".to_string();
    }

    let part = filename.rsplit('/').next().unwrap_or("");
    if part.is_empty() {
        return "$$Component".to_string();
    }

    let stem = part.split('.').next().unwrap_or(part);
    if stem.is_empty() {
        return "$$Component".to_string();
    }

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
    options.typescript.only_remove_type_imports = true;
    let _ = oxc_transformer::Transformer::new(allocator, std::path::Path::new(""), &options)
        .build_with_scoping(scoping, &mut program);

    let codegen_options = oxc_codegen::CodegenOptions {
        single_quote: false,
        ..oxc_codegen::CodegenOptions::default()
    };
    oxc_codegen::Codegen::new()
        .with_options(codegen_options)
        .build(&program)
        .code
}

#[cfg(test)]
mod tests {
    use super::*;
    use oxc_allocator::Allocator;
    use oxc_parser::Parser;
    use oxc_span::SourceType;

    fn compile_astro(source: &str) -> String {
        compile_astro_with_options(
            source,
            TransformOptions::new().with_internal_url("http://localhost:3000/"),
        )
        .code
    }

    fn compile_astro_with_options(source: &str, options: TransformOptions) -> TransformResult {
        let allocator = Allocator::default();
        let source_type = SourceType::astro();
        let ret = Parser::new(&allocator, source, source_type).parse_astro();
        assert!(ret.errors.is_empty(), "Parse errors: {:?}", ret.errors);

        transform(&allocator, source, options, &ret.root)
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
        let source = r"---
const href = '/about';
---
<a href={href}>About</a>";
        let output = compile_astro(source);

        assert!(
            output.contains("const href = \"/about\""),
            "Missing const declaration"
        );
        assert!(
            output.contains("$$addAttribute(href, \"href\")"),
            "Missing $$addAttribute"
        );
    }

    #[test]
    fn test_component_rendering() {
        let source = r"---
import Component from 'test';
---
<Component />";
        let output = compile_astro(source);

        assert!(
            output.contains("import Component from \"test\""),
            "Missing import"
        );
        assert!(
            output.contains("$$renderComponent"),
            "Missing $$renderComponent"
        );
        assert!(output.contains("\"Component\""), "Missing component name");
    }

    #[test]
    fn test_doctype() {
        let source = "<!DOCTYPE html><div></div>";
        let output = compile_astro(source);

        assert!(output.contains("<div></div>"), "Missing div element");
        assert!(
            output.contains("$$maybeRenderHead"),
            "Missing maybeRenderHead"
        );
    }

    #[test]
    fn test_fragment() {
        let source = "<><div>1</div><div>2</div></>";
        let output = compile_astro(source);

        assert!(
            output.contains("$$renderComponent"),
            "Missing renderComponent"
        );
        assert!(output.contains("Fragment"), "Missing Fragment reference");
        assert!(output.contains("<div>1</div>"), "Missing first div");
        assert!(output.contains("<div>2</div>"), "Missing second div");
    }

    #[test]
    fn test_html_head_body() {
        let source = r"<html>
  <head>
    <title>Test</title>
  </head>
  <body>
    <h1>Hello</h1>
  </body>
</html>";
        let output = compile_astro(source);

        assert!(
            output.contains("$$renderHead($$result)"),
            "Missing renderHead in head"
        );
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

        assert!(
            output.contains("$$addAttribute(src, \"src\")"),
            "Missing dynamic src attribute"
        );
        assert!(
            output.contains("alt=\"test\""),
            "Missing static alt attribute"
        );
    }

    #[test]
    fn test_expression_in_content() {
        let source = r#"---
const name = "World";
---
<h1>Hello {name}!</h1>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("Hello ${name}!"),
            "Missing interpolated expression"
        );
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
        let source = r#"---
import Component from "test";
---
<Component>{value && <div slot="test">foo</div>}</Component>"#;
        let output = compile_astro(source);

        assert!(output.contains("\"test\":"), "Missing named slot 'test'");
        assert!(
            !output.contains("slot=\"test\""),
            "Slot attribute should be removed from element"
        );
        assert!(
            output.contains("<div>foo</div>"),
            "Missing div element without slot attr"
        );
    }

    #[test]
    fn test_expression_slot_multiple() {
        let source = r#"---
import Component from "test";
---
<Component>{true && <div slot="a">A</div>}{false && <div slot="b">B</div>}</Component>"#;
        let output = compile_astro(source);

        assert!(output.contains("\"a\":"), "Missing named slot 'a'");
        assert!(output.contains("\"b\":"), "Missing named slot 'b'");
        assert!(
            !output.contains("\"default\":"),
            "Should not have default slot"
        );
    }

    #[test]
    fn test_client_load_directive() {
        let source = r"---
import Component from 'test';
---
<Component client:load />";
        let output = compile_astro(source);

        assert!(
            output.contains("client:component-hydration") && output.contains("load"),
            "Missing hydration directive"
        );
    }

    #[test]
    fn test_void_elements() {
        let source = r#"<meta charset="utf-8"><input type="text"><br><img src="x.png"><link rel="stylesheet" href="style.css"><hr>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("<meta charset=\"utf-8\">"),
            "Missing meta tag"
        );
        assert!(
            output.contains("<input type=\"text\">"),
            "Missing input tag"
        );
        assert!(output.contains("<br>"), "Missing br tag");
        assert!(output.contains("<img"), "Missing img tag");
        assert!(output.contains("<link"), "Missing link tag");
        assert!(output.contains("<hr>"), "Missing hr tag");

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
        let source = r"<html>
  <head>
    <title>Test</title>
  </head>
  <body>
    <main>
      <h1>Hello</h1>
    </main>
  </body>
</html>";
        let output = compile_astro(source);

        assert!(
            output.contains("$$renderHead($$result)"),
            "Missing $$renderHead in head"
        );

        let maybe_render_head_count = output.matches("$$maybeRenderHead").count();
        assert_eq!(
            maybe_render_head_count, 1,
            "$$maybeRenderHead should only appear once (in import), found {maybe_render_head_count} times. Body should not have $$maybeRenderHead when explicit <head> exists"
        );
    }

    #[test]
    fn test_head_elements_skip_maybe_render_head() {
        let source = r#"<Component /><link href="style.css"><meta charset="utf-8"><script src="app.js"></script>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("<link href=\"style.css\">"),
            "Missing link element"
        );
        assert!(
            output.contains("<meta charset=\"utf-8\">"),
            "Missing meta element"
        );

        let maybe_render_head_count = output.matches("$$maybeRenderHead").count();
        assert_eq!(
            maybe_render_head_count, 1,
            "$$maybeRenderHead should only appear once (in import), found {maybe_render_head_count} times. Head elements should not trigger $$maybeRenderHead"
        );
    }

    #[test]
    fn test_custom_element() {
        let source = r#"<my-element foo="bar"></my-element>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("$$renderComponent"),
            "Custom elements should use $$renderComponent"
        );
        assert!(
            output.contains("\"my-element\"") && output.matches("\"my-element\"").count() >= 2,
            "Custom element should have tag name as both display name and quoted identifier"
        );

        let maybe_render_head_count = output.matches("$$maybeRenderHead").count();
        assert_eq!(
            maybe_render_head_count, 1,
            "$$maybeRenderHead should only appear once (in import), custom elements should not trigger it"
        );

        assert!(
            !output.contains("<my-element"),
            "Custom elements should not be rendered as HTML tags"
        );
    }

    #[test]
    fn test_html_comments_preserved() {
        let source = r#"<!-- Global Metadata -->
<meta charset="utf-8">
<!-- Another comment -->
<link rel="icon" href="/favicon.ico" />"#;
        let output = compile_astro(source);

        assert!(
            output.contains("<!-- Global Metadata -->"),
            "Missing first HTML comment"
        );
        assert!(
            output.contains("<!-- Another comment -->"),
            "Missing second HTML comment"
        );
        assert!(
            output.contains("<meta charset=\"utf-8\">"),
            "Missing meta tag"
        );
        assert!(
            output.contains("<link rel=\"icon\" href=\"/favicon.ico\">"),
            "Missing link tag"
        );
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
            TransformOptions::new().with_filename("/users/astro/apps/pacman/src/pages/index.astro"),
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
            TransformOptions::new().with_filename("/users/astro/apps/pacman/src/pages/index.astro"),
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

        assert!(result.propagation, "server:defer should enable propagation");
    }

    #[test]
    fn test_contains_head_metadata() {
        let source = r"<html>
<head><title>Test</title></head>
<body><p>content</p></body>
</html>";
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

        assert!(
            !result.contains_head,
            "Should not detect <head> when absent"
        );
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

        assert_eq!(result.hydrated_components.len(), 1);
        let c = &result.hydrated_components[0];
        assert_eq!(c.specifier, "../components/Counter.jsx");
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

        assert!(
            !result.code.contains("$$createMetadata"),
            "Should skip $$createMetadata"
        );
        assert!(
            !result.code.contains("$$metadata"),
            "Should skip $$metadata export"
        );
        assert!(
            !result.code.contains("$$module1"),
            "Should skip $$module imports"
        );
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

        assert_eq!(result.hydrated_components.len(), 1);
        assert_eq!(result.hydrated_components[0].resolved_path, "some-package");
    }

    #[test]
    fn test_server_defer_skips_attribute() {
        let source = r"---
import Avatar from './Avatar.jsx';
---
<Avatar server:defer />";
        let result = compile_astro_with_options(
            source,
            TransformOptions::new().with_internal_url("http://localhost:3000/"),
        );

        assert!(
            !result.code.contains("\"server:defer\""),
            "server:defer should be stripped from props"
        );
    }

    #[test]
    fn test_typescript_satisfies_stripped() {
        let source = r"---
interface SEOProps { title: string; }
const seo = { title: 'Hello' } satisfies SEOProps;
---
<h1>{seo.title}</h1>";
        let output = compile_astro(source);

        assert!(
            !output.contains("satisfies"),
            "satisfies keyword should be stripped: {output}"
        );
        assert!(
            !output.contains("interface SEOProps"),
            "interface should be stripped: {output}"
        );
        assert!(
            output.contains("title: \"Hello\"") || output.contains("title: 'Hello'"),
            "value expression should remain: {output}"
        );
    }

    #[test]
    fn test_type_only_import_stripped() {
        let source = r"---
import type { Props } from './types';
const x: Props = { title: 'hi' };
---
<h1>{x.title}</h1>";
        let output = compile_astro(source);

        assert!(
            !output.contains("import type"),
            "import type should be stripped: {output}"
        );
    }
}
