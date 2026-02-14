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
use rustc_hash::{FxHashMap, FxHashSet};

use oxc_codegen::{Codegen, CodegenOptions, Context, Gen, GenExpr};
use oxc_span::{GetSpan, Span};
use oxc_syntax::precedence::Precedence;

use crate::{SourcemapOption, TransformOptions};
use crate::css_scoping;
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

mod sourcemap_builder;
#[cfg(test)]
mod sourcemap_tests;

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

// Bring sourcemap builder into scope.
use sourcemap_builder::AstroSourcemapBuilder;

// --- Codegen helpers ---
//
// These free functions wrap `oxc_codegen::Codegen` to convert AST nodes into
// source-text strings.  They replace the repetitive
//     `Codegen::new()` → `print_expr` / `print` → `into_source_text()`
// pattern that was duplicated 10+ times across the printer submodules.

/// Convert an expression AST node to a JavaScript source string.
pub fn expr_to_string(expr: &(impl GenExpr + ?Sized)) -> String {
    let mut codegen = Codegen::new();
    expr.print_expr(
        &mut codegen,
        Precedence::Lowest,
        Context::default().with_typescript(),
    );
    codegen.into_source_text()
}

/// Convert an AST node that implements [`Gen`] (e.g. `Statement`,
/// `VariableDeclaration`, `BindingPattern`) to a JavaScript source string.
pub fn gen_to_string(node: &(impl Gen + ?Sized)) -> String {
    let mut codegen = Codegen::new();
    node.print(&mut codegen, Context::default().with_typescript());
    codegen.into_source_text()
}

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

/// Metadata about an extractable `<style>` block in an Astro component.
///
/// Returned by [`extract_styles`] so that callers (e.g. the TS wrapper) can
/// preprocess style content before compilation.
#[derive(Debug, Clone)]
pub struct StyleBlock {
    /// Zero-based index of this style block among all extractable styles.
    pub index: usize,
    /// The CSS/preprocessor text content between `<style>` and `</style>`.
    pub content: String,
    /// The element's attributes as key-value pairs.
    /// Only quoted and empty (boolean) attributes are included — expression
    /// attributes (like `define:vars={...}`) are omitted, matching the Go
    /// compiler's `GetAttrs` behavior.
    pub attrs: Vec<(String, String)>,
}

/// Extract style block metadata from an Astro AST without performing compilation.
///
/// This walks the template to find all extractable `<style>` elements (i.e.
/// those that are **not** `is:inline`, `set:html`, `set:text`, or inside
/// non-hoistable contexts like `<svg>`/`<noscript>`/`<template>`).
///
/// For each extractable style, it returns a [`StyleBlock`] with the text
/// content and attributes. The blocks are returned in document order and
/// their `index` fields are sequential (0, 1, 2, …).
///
/// This is the first step in the "Rust extract → TS preprocess → Rust compile"
/// pipeline for `preprocessStyle` support.
pub fn extract_styles<'a>(root: &'a AstroRoot<'a>) -> Vec<StyleBlock> {
    let mut blocks = Vec::new();
    extract_styles_from_children(&root.body, false, &mut blocks);
    blocks
}

fn extract_styles_from_children<'a>(
    children: &[JSXChild<'a>],
    in_non_hoistable: bool,
    blocks: &mut Vec<StyleBlock>,
) {
    for child in children {
        match child {
            JSXChild::Element(el) => {
                let name = get_jsx_element_name(&el.opening_element.name);
                if name == "style" && !in_non_hoistable && should_extract_style_element(el) {
                    // Extract content and attrs
                    let content = extract_style_text(el);
                    let attrs = extract_style_attrs(el);
                    blocks.push(StyleBlock {
                        index: blocks.len(),
                        content,
                        attrs,
                    });
                } else {
                    let non_hoistable = in_non_hoistable
                        || matches!(name.as_str(), "svg" | "noscript" | "template");
                    extract_styles_from_children(&el.children, non_hoistable, blocks);
                }
            }
            JSXChild::Fragment(frag) => {
                extract_styles_from_children(&frag.children, in_non_hoistable, blocks);
            }
            _ => {}
        }
    }
}

/// Check if a `<style>` element should be extracted (same logic as
/// `AstroCodegen::should_extract_style` but as a free function).
fn should_extract_style_element(el: &JSXElement<'_>) -> bool {
    let attrs = &el.opening_element.attributes;
    for attr in attrs {
        if let JSXAttributeItem::Attribute(attr) = attr {
            let name = get_jsx_attribute_name(&attr.name);
            if name == "is:inline" || name == "set:html" || name == "set:text" {
                return false;
            }
        }
    }
    true
}

/// Extract the text content of a `<style>` element.
fn extract_style_text(el: &JSXElement<'_>) -> String {
    let mut text = String::new();
    for child in &el.children {
        if let JSXChild::Text(t) = child {
            text.push_str(t.value.as_str());
        }
    }
    text
}

/// Extract quoted and empty (boolean) attributes from a `<style>` element.
/// Expression attributes (like `define:vars={...}`) are omitted, matching
/// the Go compiler's `GetAttrs` behavior.
fn extract_style_attrs(el: &JSXElement<'_>) -> Vec<(String, String)> {
    let mut attrs = Vec::new();
    for attr_item in &el.opening_element.attributes {
        if let JSXAttributeItem::Attribute(attr) = attr_item {
            let name = get_jsx_attribute_name(&attr.name);
            match &attr.value {
                None => {
                    // Boolean/empty attribute (e.g. `is:global`)
                    attrs.push((name, String::new()));
                }
                Some(JSXAttributeValue::StringLiteral(lit)) => {
                    // Quoted attribute (e.g. `lang="scss"`)
                    attrs.push((name, lit.value.as_str().to_string()));
                }
                _ => {
                    // Expression attributes are skipped
                }
            }
        }
    }
    attrs
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
    /// Sourcemap builder (present when `options.sourcemap` is enabled)
    sourcemap_builder: Option<AstroSourcemapBuilder<'a>>,
    /// Collected CSS strings from extracted `<style>` elements (scoped).
    /// Each entry corresponds to one `<style>` tag.
    extracted_css: Vec<String>,
    /// Whether any style was scoped (i.e., at least one non-global, non-inline style exists).
    has_scoped_styles: bool,
    /// Tracks whether we're inside an element that prevents style hoisting
    /// (svg, noscript, template).
    in_non_hoistable: bool,
    /// Collected `define:vars` expression values from `<style>` elements.
    /// Each entry is the raw JS expression string (e.g., `{color:'green'}`).
    define_vars_values: Vec<String>,
    /// Whether any element has received `$$definedVars` style injection.
    define_vars_injected: bool,
    /// Counter for the current extractable style index during prescan.
    /// Used to look up preprocessed style content from `options.preprocessed_styles`.
    style_extraction_index: usize,
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
        // Compute base hash for the source file.
        // Use normalizedFilename (like Go's compiler) so that different files
        // with identical content get different scope hashes.  Fall back to
        // source text only when no filename is supplied (i.e. "<stdin>").
        let hash_input = options
            .normalized_filename
            .as_deref()
            .or(options.filename.as_deref())
            .filter(|s| *s != "<stdin>")
            .unwrap_or(source_text);
        let source_hash = Self::compute_source_hash(hash_input);

        // Initialize sourcemap builder if requested
        let sourcemap_builder = if options.sourcemap.is_enabled() {
            let path = options.filename.as_deref().unwrap_or("<stdin>");
            Some(AstroSourcemapBuilder::new(
                std::path::Path::new(path),
                source_text,
            ))
        } else {
            None
        };

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
            sourcemap_builder,
            extracted_css: Vec::new(),
            has_scoped_styles: false,
            in_non_hoistable: false,
            define_vars_values: Vec::new(),
            define_vars_injected: false,
            style_extraction_index: 0,
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
        if let Some(dot_pos) = component_name.find('.') {
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

    // --- Shared arrow-function helpers ---

    /// Print arrow function parameters including parentheses and the `=>` arrow.
    ///
    /// Prints the async prefix (if applicable), the parameter list (with
    /// optional parentheses for single-identifier params), and ` => `.
    /// The caller is responsible for printing the body.
    fn print_arrow_params(&mut self, arrow: &ArrowFunctionExpression<'a>) {
        if arrow.r#async {
            self.print("async ");
        }
        // Single simple identifier param doesn't need parens, but destructuring patterns do
        let needs_parens = arrow.params.items.len() != 1
            || arrow.params.rest.is_some()
            || !matches!(
                arrow.params.items.first().map(|p| &p.pattern),
                Some(BindingPattern::BindingIdentifier(_))
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
    }

    // --- Sourcemap helpers ---

    /// Record a sourcemap mapping for a `Span` (uses `span.start`).
    fn add_source_mapping_for_span(&mut self, span: Span) {
        if let Some(ref mut sm) = self.sourcemap_builder {
            sm.add_source_mapping_for_span(self.code.as_bytes(), span);
        }
    }

    /// Print a multi-line string and record a source mapping at the start of
    /// each line so that every intermediate line has a Phase 1 token.
    ///
    /// This is needed for expressions that `oxc_codegen::Codegen` expands
    /// across multiple lines (e.g. `[1, 2, 3]` → three lines).  Without
    /// per-line mappings, Phase 2 composition's `lookup_token` (which only
    /// searches within a single line) returns `None` for those lines and the
    /// mapping is lost.
    fn print_multiline_with_mappings(&mut self, text: &str, span: Span) {
        if span.is_empty() || self.sourcemap_builder.is_none() {
            // No sourcemap — just print the text directly.
            self.print(text);
            return;
        }

        let mut first = true;
        for line in text.split('\n') {
            if !first {
                self.code.print_char('\n');
                // Record a mapping at the start of this new line, pointing
                // back to the expression's original span.  Uses `_force` to
                // bypass the consecutive-position dedup (all lines map to the
                // same original byte offset).
                if let Some(ref mut sm) = self.sourcemap_builder {
                    sm.add_source_mapping_force(self.code.as_bytes(), span.start);
                }
            }
            first = false;
            self.code.print_str(line);
        }
    }

    // --- Build ---

    /// Build the JavaScript output from an Astro AST.
    ///
    /// # Panics
    ///
    /// Panics if the intermediate or final code exceeds `isize::MAX` lines
    /// (impossible in practice for source files).
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
        let intermediate_code = self.code.into_string();
        let phase1_sourcemap = self.sourcemap_builder.take();
        let source_path = self.options.filename.as_deref().unwrap_or("<stdin>");

        // Strip TypeScript and compose sourcemaps.
        let (mut code, sourcemap) = strip_and_compose_sourcemaps(
            self.allocator,
            &intermediate_code,
            phase1_sourcemap,
            source_path,
            self.source_text,
        );

        // Apply sourcemap mode: inline, both, or external.
        let sourcemap_mode = self.options.sourcemap;
        let map = match (sourcemap, sourcemap_mode) {
            (Some(sm), SourcemapOption::Inline) => {
                code.push_str("\n//# sourceMappingURL=");
                code.push_str(&sm.to_data_url());
                String::new()
            }
            (Some(sm), SourcemapOption::Both) => {
                let json = sm.to_json_string();
                code.push_str("\n//# sourceMappingURL=");
                code.push_str(&sm.to_data_url());
                json
            }
            (Some(sm), _) => sm.to_json_string(),
            (None, _) => String::new(),
        };

        let css = std::mem::take(&mut self.extracted_css);

        TransformResult {
            code,
            map,
            scope,
            style_error: Vec::new(),
            diagnostics: Vec::new(),
            css,
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
        // Pre-scan: extract styles from the template so we know the CSS count
        // before printing. This walks the AST to find <style> elements,
        // extracts their CSS, applies scoping, and stores in self.extracted_css.
        self.prescan_styles(&root.body);

        // 1. Print internal imports
        self.print_internal_imports();

        // 1b. Print CSS imports (one per extracted style)
        self.print_css_imports();

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
            let url = self
                .options
                .transitions_animation_url
                .as_deref()
                .unwrap_or("transitions.css");
            self.println(&format!("import \"{url}\";"));
        }
    }

    /// Pre-scan the template body to extract `<style>` elements.
    /// This populates `self.extracted_css` and `self.has_scoped_styles`
    /// before any printing happens.
    fn prescan_styles(&mut self, body: &[JSXChild<'a>]) {
        for child in body {
            self.prescan_styles_child(child, false);
        }
    }

    fn prescan_styles_child(&mut self, child: &JSXChild<'a>, in_non_hoistable: bool) {
        match child {
            JSXChild::Element(el) => {
                let name = get_jsx_element_name(&el.opening_element.name);
                if name == "style" && !in_non_hoistable && self.should_extract_style(el) {
                    self.extract_style_element(el);
                } else {
                    // Mark descendant context as non-hoistable for svg/noscript/template
                    let non_hoistable = in_non_hoistable
                        || matches!(name.as_str(), "svg" | "noscript" | "template");
                    for c in &el.children {
                        self.prescan_styles_child(c, non_hoistable);
                    }
                }
            }
            JSXChild::Fragment(frag) => {
                for c in &frag.children {
                    self.prescan_styles_child(c, in_non_hoistable);
                }
            }
            _ => {}
        }
    }

    /// Print CSS import statements, one per extracted `<style>` tag.
    /// Format: `import "filename?astro&type=style&index=N&lang.css";`
    fn print_css_imports(&mut self) {
        if self.extracted_css.is_empty() {
            return;
        }
        let filename = self
            .options
            .filename
            .clone()
            .unwrap_or_else(|| "<stdin>".to_string());

        for i in 0..self.extracted_css.len() {
            self.println(&format!(
                "import \"{filename}?astro&type=style&index={i}&lang.css\";"
            ));
        }
    }

    fn print_namespace_imports(&mut self) {
        if self.module_imports.is_empty() || self.options.has_resolve_path() {
            return;
        }

        for i in 0..self.module_imports.len() {
            if self.module_imports[i].assertion == "{}" {
                let line = format!(
                    "import * as {} from \"{}\";",
                    self.module_imports[i].namespace_var, self.module_imports[i].specifier
                );
                self.println(&line);
            } else {
                let line = format!(
                    "import * as {} from \"{}\" assert {};",
                    self.module_imports[i].namespace_var,
                    self.module_imports[i].specifier,
                    self.module_imports[i].assertion
                );
                self.println(&line);
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
                let mut seen = FxHashSet::default();
                let mut items = Vec::new();
                for h in scan_client_only {
                    let root = if let Some(dot_pos) = h.name.find('.') {
                        &h.name[..dot_pos]
                    } else {
                        h.name.as_str()
                    };
                    if let Some(info) = self.component_imports.get(root)
                        && seen.insert(info.specifier.clone())
                    {
                        items.push(format!("\"{}\"", info.specifier));
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
        let span = stmt.span();
        let raw = &self.source_text[span.start as usize..span.end as usize];
        let raw = raw.trim_end_matches('\n');

        // First line gets the normal span mapping.
        self.add_source_mapping_for_span(span);

        let mut offset: u32 = 0;
        let mut first = true;
        for line in raw.split('\n') {
            if !first {
                self.code.print_char('\n');
                // Map this line to its actual position in the original source.
                if let Some(ref mut sm) = self.sourcemap_builder {
                    sm.add_source_mapping_force(self.code.as_bytes(), span.start + offset);
                }
            }
            first = false;
            self.code.print_str(line);
            // +1 for the '\n' delimiter between lines.
            // Line is a substring of a Span, which is bounded by u32.
            offset += u32::try_from(line.len()).expect("line length exceeds u32") + 1;
        }
        self.code.print_char('\n');
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

        // Emit $$definedVars declaration if define:vars is present on any <style>
        if !self.define_vars_values.is_empty() {
            let joined = self.define_vars_values.join(",");
            self.println(&format!(
                "const $$definedVars = {}([{}]);",
                runtime::DEFINE_STYLE_VARS,
                joined
            ));
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
        self.add_source_mapping_for_span(comment.span);
        self.print("<!--");
        self.print(&escape_template_literal(comment.value.as_str()));
        self.print("-->");
    }

    fn print_jsx_text(&mut self, text: &JSXText<'a>) {
        self.add_source_mapping_for_span(text.span);
        let escaped = escape_template_literal(text.value.as_str());
        self.print(&escaped);
    }

    /// Dispatch a JSX element to either component or HTML element printing.
    fn print_jsx_element(&mut self, el: &JSXElement<'a>) {
        let name = get_jsx_element_name(&el.opening_element.name);

        // Handle <style> elements — extract CSS, skip from template output
        // (only if not inside svg/noscript/template)
        if name == "style" && !self.in_non_hoistable && self.should_extract_style(el) {
            // Style was already extracted during prescan — just skip it from template
            return;
        }

        // Handle <script> elements that should be hoisted
        if name == "script" && Self::is_hoisted_script(el) {
            self.add_source_mapping_for_span(el.opening_element.span);
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

    /// Check if a `<style>` element should be extracted (removed from template
    /// and its CSS emitted separately).
    ///
    /// A `<style>` is extracted unless:
    /// - It has `is:inline` (render as raw HTML)
    /// - It has `set:html` or `set:text` (directive-driven content)
    /// - It is inside an `<svg>`, `<noscript>`, or other non-hoistable context
    ///   (approximated by checking element_depth — styles at depth 0 are always
    ///   hoisted; the Go compiler has a more precise `IsHoistable` check)
    fn should_extract_style(&self, el: &JSXElement<'a>) -> bool {
        let attrs = &el.opening_element.attributes;

        // Don't extract if has is:inline
        if Self::has_is_inline_attribute(attrs) {
            return false;
        }

        // Don't extract if has set:html or set:text
        if Self::extract_set_directive(attrs).is_some() {
            return false;
        }

        true
    }

    /// Extract a `<style>` element: collect its CSS content, apply scoping,
    /// and skip the element from the template output.
    fn extract_style_element(&mut self, el: &JSXElement<'a>) {
        // Capture the current extraction index and bump it for the next style
        let current_index = self.style_extraction_index;
        self.style_extraction_index += 1;

        // Check for is:global attribute
        let is_global = el.opening_element.attributes.iter().any(|attr| {
            if let JSXAttributeItem::Attribute(attr) = attr {
                get_jsx_attribute_name(&attr.name) == "is:global"
            } else {
                false
            }
        });

        // Check for define:vars attribute and collect its value
        let has_define_vars = self.collect_define_vars(el);

        // Get CSS content: use preprocessed content if available, otherwise read from AST.
        let css_content = if let Some(preprocessed) = self
            .options
            .preprocessed_styles
            .as_ref()
            .and_then(|styles| styles.get(current_index))
            .and_then(|entry| entry.as_ref())
        {
            preprocessed.clone()
        } else {
            self.extract_style_text_content(el)
        };
        let css_trimmed = css_content.trim();

        if css_trimmed.is_empty() {
            // Empty style — still counts for scoping purposes if not global
            // or if it has define:vars
            if !is_global || has_define_vars {
                self.has_scoped_styles = true;
            }
            return;
        }

        // Apply CSS scoping (unless is:global)
        let scoped_css = if is_global {
            css_trimmed.to_string()
        } else {
            self.has_scoped_styles = true;
            css_scoping::scope_css(
                css_trimmed,
                &self.source_hash,
                self.options.scoped_style_strategy(),
            )
        };

        self.extracted_css.push(scoped_css);
    }

    /// Check for `define:vars` attribute on a `<style>` element and collect its value.
    /// Returns `true` if `define:vars` was found.
    fn collect_define_vars(&mut self, el: &JSXElement<'a>) -> bool {
        for attr in &el.opening_element.attributes {
            if let JSXAttributeItem::Attribute(attr) = attr {
                let name = get_jsx_attribute_name(&attr.name);
                if name == "define:vars" {
                    match &attr.value {
                        Some(JSXAttributeValue::StringLiteral(lit)) => {
                            self.define_vars_values
                                .push(format!("'{}'", lit.value.as_str()));
                        }
                        Some(JSXAttributeValue::ExpressionContainer(expr)) => {
                            if let Some(e) = expr.expression.as_expression() {
                                self.define_vars_values.push(expr_to_string(e));
                            }
                        }
                        _ => {}
                    }
                    return true;
                }
            }
        }
        false
    }

    /// Extract the text content of a `<style>` element.
    fn extract_style_text_content(&self, el: &JSXElement<'a>) -> String {
        let mut text = String::new();
        for child in &el.children {
            if let JSXChild::Text(t) = child {
                text.push_str(t.value.as_str());
            }
        }
        text
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

/// A sourcemap token used to collect, sort, and deduplicate all mappings
/// before feeding them to the sourcemap builder (which requires tokens in
/// ascending generated-position order).
struct RawToken {
    dst_line: u32,
    dst_col: u32,
    src_line: u32,
    src_col: u32,
    name: Option<String>,
}

/// Strip TypeScript from `intermediate_code` and compose the resulting
/// sourcemap with the Phase 1 sourcemap from Astro codegen.
///
/// Phase 1 maps intermediate positions → original `.astro` positions.
/// Phase 2 (TypeScript stripping / oxc_codegen re-emit) maps final positions
/// → intermediate positions.  Composition gives us final positions → original
/// `.astro` positions.
///
/// However, Phase 2 only produces tokens at AST node boundaries.  The template
/// literal `$$render\`...\`` is one AST node, so all the fine-grained Phase 1
/// tokens *inside* it are lost during naïve composition.  After composing, this
/// function carries forward Phase 1 tokens that were not covered by Phase 2 by
/// computing line/column adjustments between the intermediate and final code.
///
/// Returns `(final_code, Option<SourceMap>)` where the sourcemap is `None` if
/// no sourcemap was requested.
///
/// # Panics
///
/// Panics if line or column values exceed `u32` or `i64` (impossible in
/// practice for source files).
fn strip_and_compose_sourcemaps(
    allocator: &Allocator,
    intermediate_code: &str,
    phase1_sourcemap: Option<AstroSourcemapBuilder<'_>>,
    source_path: &str,
    source_text: &str,
) -> (String, Option<oxc_sourcemap::SourceMap>) {
    let generate_sourcemap = phase1_sourcemap.is_some();
    let (code, phase2_map) = strip_typescript(allocator, intermediate_code, generate_sourcemap);

    let map = if let (Some(phase2_map), Some(phase1_sm)) = (phase2_map, phase1_sourcemap) {
        let phase1_map = phase1_sm.into_sourcemap();
        let composed =
            sourcemap_builder::remap_sourcemap(&phase2_map, &phase1_map, source_path, source_text);

        // Supplement: carry forward Phase 1 template-literal tokens,
        // but ONLY on lines where the intermediate and final content
        // match (same text, possibly differing only in leading whitespace).
        //
        // Phase 2 (oxc_codegen) reformats expressions inside template
        // literal interpolations (e.g. adds parens, changes indentation),
        // so columns on those lines differ between intermediate and final.
        // Supplementing such lines with Phase 1 column positions produces
        // wrong mappings.
        //
        // However, oxc_codegen commonly adds a leading tab to lines inside
        // function bodies while leaving the rest of the line identical.
        // We detect this case and adjust column offsets accordingly, so
        // Phase 1 tokens on those lines are preserved with correct columns.
        //
        // For lines that truly differ (reformatted expressions, added
        // parens, etc.), composition (Phase 2 → Phase 1 lookup) already
        // provides correct mappings at AST-node granularity.
        let inter_lines: Vec<&str> = intermediate_code.lines().collect();
        let final_lines: Vec<&str> = code.lines().collect();

        let inter_template_line = inter_lines
            .iter()
            .position(|l| l.contains("return $$render`"));
        let final_template_line = final_lines
            .iter()
            .position(|l| l.contains("return $$render`"));

        let composed = if let (Some(i_line), Some(f_line)) =
            (inter_template_line, final_template_line)
        {
            // For each intermediate line, compute the column adjustment
            // needed to map intermediate columns to final columns.
            //
            // - If the lines match exactly: delta is 0.
            // - If they differ only in leading whitespace: uniform delta.
            // - If they differ due to `</` → `<\/` escaping (template
            //   literal safety): leading-ws delta plus per-position
            //   adjustments for each inserted backslash.
            //
            // `None` means the lines genuinely differ and supplementing
            // is not safe.
            //
            // The value is (ws_delta, escape_insert_positions) where
            // escape_insert_positions lists intermediate columns where
            // a `\` was inserted in the final text (empty for exact or
            // whitespace-only matches).
            let line_col_info: FxHashMap<usize, (i64, Vec<usize>)> = (i_line..inter_lines.len())
                .filter_map(|il| {
                    // il >= i_line is guaranteed by the range, so this
                    // never underflows.  f_line + (il - i_line) is the
                    // corresponding final line index.
                    let fl = il - i_line + f_line;
                    if fl >= final_lines.len() {
                        return None;
                    }
                    let i_text = inter_lines[il];
                    let f_text = final_lines[fl];

                    // Fast path: lines are identical.
                    if i_text == f_text {
                        return Some((il, (0i64, Vec::new())));
                    }

                    // Check if lines differ only in leading whitespace
                    // (and/or template literal escaping like <\/ vs </).
                    let i_trimmed = i_text.trim_start();
                    let f_trimmed = f_text.trim_start();

                    // Leading whitespace counts are bounded by line
                    // length which is bounded by file size — always
                    // representable as i64.
                    let i_ws_len = i64::try_from(i_text.len() - i_trimmed.len())
                        .expect("whitespace length exceeds i64");
                    let f_ws_len = i64::try_from(f_text.len() - f_trimmed.len())
                        .expect("whitespace length exceeds i64");
                    let ws_delta = f_ws_len - i_ws_len;

                    if i_trimmed == f_trimmed {
                        return Some((il, (ws_delta, Vec::new())));
                    }

                    // Normalize template literal escapes: oxc_codegen
                    // escapes `</` as `<\/` inside template literals for
                    // HTML safety.  We treat these as matching, but track
                    // the insertion positions for column adjustment.
                    let i_norm = i_trimmed.replace("<\\/", "</");
                    let f_norm = f_trimmed.replace("<\\/", "</");
                    if i_norm != f_norm {
                        return None; // Content genuinely differs.
                    }

                    // Find positions in the intermediate trimmed text
                    // where `</` occurs — these correspond to `<\/` in
                    // the final text, meaning a `\` was inserted after
                    // the `<`.  We record the intermediate column (in
                    // the full line, not trimmed) of the `/` char, which
                    // is where the shift starts.
                    //
                    // We search `i_trimmed` (intermediate) for `</`
                    // rather than `f_trimmed` for `<\/`, because the
                    // intermediate positions are what we need and
                    // searching the final text would give wrong offsets
                    // for the 2nd+ escape on the same line (each `<\/`
                    // is 3 chars in final but only 2 in intermediate,
                    // causing a cumulative +1 error per prior escape).
                    let mut escape_positions = Vec::new();
                    let i_ws = i_text.len() - i_trimmed.len();
                    let mut search_start = 0;
                    let search_text = i_trimmed;
                    while let Some(pos) = search_text[search_start..].find("</") {
                        let trimmed_pos = search_start + pos;
                        // The `\` is inserted at trimmed_pos + 1 (after `<`).
                        // In intermediate line coords, this corresponds to
                        // i_ws + trimmed_pos + 1 (the `/` in intermediate).
                        escape_positions.push(i_ws + trimmed_pos + 1);
                        search_start = trimmed_pos + 2; // skip past `</`
                    }

                    Some((il, (ws_delta, escape_positions)))
                })
                .collect();

            // Collect the composed tokens into a set of (dst_line, dst_col)
            // so we can skip Phase 1 tokens that were already covered.
            let composed_positions: FxHashSet<(u32, u32)> = composed
                .get_tokens()
                .map(|t| (t.get_dst_line(), t.get_dst_col()))
                .collect();

            // Build Phase-2 anchor index for DIFFER lines.
            //
            // Phase 2 tokens provide (inter_col → final_line, final_col)
            // pairs.  On DIFFER lines (where Phase 2 reformatted JS
            // expressions but left template quasi text intact), we can use
            // the nearest anchor to compute the column offset for Phase-1
            // tokens that sit inside quasi text regions.
            //
            // Keyed by intermediate line → vec of (inter_col, final_line,
            // final_col), sorted by inter_col ascending.
            let phase2_anchors: FxHashMap<u32, Vec<(u32, u32, u32)>> = {
                let mut map: FxHashMap<u32, Vec<(u32, u32, u32)>> = FxHashMap::default();
                for t in phase2_map.get_tokens() {
                    map.entry(t.get_src_line()).or_default().push((
                        t.get_src_col(),
                        t.get_dst_line(),
                        t.get_dst_col(),
                    ));
                }
                for v in map.values_mut() {
                    v.sort_by_key(|&(ic, _, _)| ic);
                }
                map
            };

            let ctx = SupplementContext {
                inter_lines,
                final_lines,
                i_line,
                f_line,
                phase2_anchors,
                line_col_info,
                composed_positions,
            };

            let mut all_tokens: Vec<RawToken> = Vec::new();

            // 1. Existing composed tokens.
            for t in composed.get_tokens() {
                let name = t
                    .get_name_id()
                    .and_then(|id| composed.get_name(id))
                    .map(std::string::ToString::to_string);
                all_tokens.push(RawToken {
                    dst_line: t.get_dst_line(),
                    dst_col: t.get_dst_col(),
                    src_line: t.get_src_line(),
                    src_col: t.get_src_col(),
                    name,
                });
            }

            // 2. Phase 1 tokens inside the template literal, on lines
            //    where the content matches (exactly or after leading
            //    whitespace adjustment), OR on DIFFER lines where the
            //    token sits inside a quasi text region that is identical
            //    between intermediate and final code.
            supplement_phase1_tokens(&phase1_map, &ctx, &mut all_tokens);

            // Sort by generated position (line first, then column).
            all_tokens.sort_by(|a, b| a.dst_line.cmp(&b.dst_line).then(a.dst_col.cmp(&b.dst_col)));

            // Deduplicate tokens at the same generated position.
            all_tokens.dedup_by(|a, b| a.dst_line == b.dst_line && a.dst_col == b.dst_col);

            // Build the final sourcemap in sorted order.
            let mut builder = oxc_sourcemap::SourceMapBuilder::default();
            let src_id = builder.set_source_and_content(source_path, source_text);

            for t in &all_tokens {
                let name_id = t.name.as_deref().map(|n| builder.add_name(n));
                builder.add_token(
                    t.dst_line,
                    t.dst_col,
                    t.src_line,
                    t.src_col,
                    Some(src_id),
                    name_id,
                );
            }

            builder.into_sourcemap()
        } else {
            composed
        };

        Some(composed)
    } else {
        None
    };

    (code, map)
}

/// Shared context for the intermediate-to-final line mapping used by the
/// sourcemap supplementing logic.
struct SupplementContext<'a> {
    /// Lines of the intermediate (Phase 1) code.
    inter_lines: Vec<&'a str>,
    /// Lines of the final (Phase 2) code.
    final_lines: Vec<&'a str>,
    /// Line index of `return $$render\`` in the intermediate code.
    i_line: usize,
    /// Line index of `return $$render\`` in the final code.
    f_line: usize,
    /// Phase-2 anchor index: intermediate line → vec of
    /// `(inter_col, final_line, final_col)`, sorted by `inter_col` ascending.
    phase2_anchors: FxHashMap<u32, Vec<(u32, u32, u32)>>,
    /// Per-line column adjustment info for matched lines.
    /// Key: intermediate line index.
    /// Value: `(ws_delta, escape_insert_positions)`.
    line_col_info: FxHashMap<usize, (i64, Vec<usize>)>,
    /// Set of `(dst_line, dst_col)` already covered by Phase 2 composition.
    composed_positions: FxHashSet<(u32, u32)>,
}

/// Carry forward Phase 1 tokens inside the template literal region that
/// were not covered by Phase 2 composition.
///
/// For matched lines (exact, leading-whitespace, or escape-normalized), the
/// token column is adjusted by the whitespace delta and escape shift.
///
/// For DIFFER lines (where Phase 2 reformatted JS expressions), tokens in
/// quasi text regions are carried forward using the nearest Phase-2 anchor
/// to compute the column offset, with a text-verification check.
///
/// For lines with no Phase-2 anchors (pure template quasi text), a small
/// window search finds the matching final line.
fn supplement_phase1_tokens(
    phase1_map: &oxc_sourcemap::SourceMap,
    ctx: &SupplementContext<'_>,
    all_tokens: &mut Vec<RawToken>,
) {
    for t in phase1_map.get_tokens() {
        let gen_line = t.get_dst_line() as usize;
        // Only consider tokens at or after the template start.
        if gen_line < ctx.i_line {
            continue;
        }

        let token_col = t.get_dst_col();

        // Try the matched-line path first (exact, leading-ws,
        // or escape-normalized match).  If absent, fall through
        // to anchor-based supplementing for DIFFER lines.
        let (adjusted_line, adjusted_col) =
            if let Some((ws_delta, escape_positions)) = ctx.line_col_info.get(&gen_line) {
                // gen_line >= i_line (checked above), so this
                // never underflows.
                let al = u32::try_from(gen_line - ctx.i_line + ctx.f_line)
                    .expect("adjusted line exceeds u32");

                // Count how many escape insertions occur at or
                // before this token's column.  Each `<\/` escape
                // adds 1 byte (the `\`) that shifts subsequent
                // columns.
                let tc = token_col as usize;
                let escape_shift =
                    i64::try_from(escape_positions.iter().filter(|&&pos| pos <= tc).count())
                        .expect("escape count exceeds i64");

                let ac = u32::try_from((i64::from(token_col) + ws_delta + escape_shift).max(0))
                    .expect("adjusted column exceeds u32");
                (al, ac)
            } else if let Some(result) = try_anchor_supplement(token_col, gen_line, ctx) {
                result
            } else {
                continue;
            };

        // Skip if already covered by composition.
        if ctx
            .composed_positions
            .contains(&(adjusted_line, adjusted_col))
        {
            continue;
        }

        // Skip if beyond final code.
        if (adjusted_line as usize) >= ctx.final_lines.len() {
            continue;
        }

        let name = t
            .get_name_id()
            .and_then(|id| phase1_map.get_name(id))
            .map(std::string::ToString::to_string);

        all_tokens.push(RawToken {
            dst_line: adjusted_line,
            dst_col: adjusted_col,
            src_line: t.get_src_line(),
            src_col: t.get_src_col(),
            name,
        });
    }
}

/// Try to supplement a Phase 1 token on a DIFFER line using Phase-2 anchors,
/// or by searching a window of final lines for pure quasi text.
///
/// Returns `Some((final_line, final_col))` if supplementing is possible,
/// `None` if the token should be skipped.
fn try_anchor_supplement(
    token_col: u32,
    gen_line: usize,
    ctx: &SupplementContext<'_>,
) -> Option<(u32, u32)> {
    let gen_line_u32 = u32::try_from(gen_line).ok()?;

    if let Some(anchors) = ctx.phase2_anchors.get(&gen_line_u32) {
        // Find the nearest anchor with inter_col <= token_col
        // (the last one in sorted order that doesn't exceed it).
        let nearest = anchors.iter().rev().find(|&&(ic, _, _)| ic <= token_col);
        let &(anchor_ic, anchor_fl, anchor_fc) = nearest?;
        let delta = i64::from(anchor_fc) - i64::from(anchor_ic);
        let candidate_col = u32::try_from((i64::from(token_col) + delta).max(0))
            .expect("candidate col exceeds u32");

        // Verify: the text at the candidate final position must match the
        // intermediate text at the token position.  This confirms we are in
        // a quasi region (not a reformatted expression).
        let i_text = ctx.inter_lines.get(gen_line).copied().unwrap_or("");
        let f_text = ctx
            .final_lines
            .get(anchor_fl as usize)
            .copied()
            .unwrap_or("");
        let tc = token_col as usize;
        let cc = candidate_col as usize;
        // Use a short verification window; 2 bytes is enough to confirm
        // we're at the same quasi text (e.g. "<p", "</", etc.).
        let verify_len = 2;
        let text_matches = tc + verify_len <= i_text.len()
            && cc + verify_len <= f_text.len()
            && i_text.as_bytes()[tc..tc + verify_len] == f_text.as_bytes()[cc..cc + verify_len];

        if text_matches {
            Some((anchor_fl, candidate_col))
        } else {
            None
        }
    } else {
        // No Phase-2 anchors for this intermediate line.
        //
        // This happens for lines that are pure template literal quasi text
        // (no JS expressions).  Phase-2 reformatting may shift these lines
        // by inserting extra lines for object literal properties, but the
        // text content remains identical.
        //
        // Search a window of final lines around the expected position for
        // a line containing the same text at the token's column.
        let i_text = ctx.inter_lines.get(gen_line).copied().unwrap_or("");
        let tc = token_col as usize;
        let verify_len = 2;
        if tc + verify_len > i_text.len() {
            return None;
        }
        let needle = &i_text.as_bytes()[tc..tc + verify_len];

        // Expected final line based on the template offset.
        // gen_line >= i_line (checked by caller).
        let expected_fl = gen_line - ctx.i_line + ctx.f_line;
        // Search ±5 lines around expected position to account for
        // Phase-2 reformatting inserting a few extra lines.
        let search_start = expected_fl.saturating_sub(5);
        let search_end = (expected_fl + 6).min(ctx.final_lines.len());

        for (fl, f_text) in ctx
            .final_lines
            .iter()
            .enumerate()
            .take(search_end)
            .skip(search_start)
        {
            // Check same column (the text is quasi literal,
            // so the column should be identical).
            if tc + verify_len <= f_text.len() && &f_text.as_bytes()[tc..tc + verify_len] == needle
            {
                let fl_u32 = u32::try_from(fl).expect("final line exceeds u32");
                return Some((fl_u32, token_col));
            }
            // Also check with leading whitespace adjustment:
            // Phase-2 may have added or removed leading ws.
            let f_trimmed = f_text.trim_start();
            let i_trimmed = i_text.trim_start();
            let f_ws = i64::try_from(f_text.len() - f_trimmed.len()).unwrap_or(0);
            let i_ws = i64::try_from(i_text.len() - i_trimmed.len()).unwrap_or(0);
            let ws_adj = f_ws - i_ws;
            let adj_col_i64 = (i64::from(token_col) + ws_adj).max(0);
            let adj_col = usize::try_from(adj_col_i64).expect("adjusted column exceeds usize");
            if adj_col + verify_len <= f_text.len()
                && &f_text.as_bytes()[adj_col..adj_col + verify_len] == needle
            {
                let fl_u32 = u32::try_from(fl).expect("final line exceeds u32");
                let ac = u32::try_from(adj_col).expect("adjusted col exceeds u32");
                return Some((fl_u32, ac));
            }
        }

        None
    }
}

/// Strip TypeScript syntax from generated code.
///
/// Parses the code as TypeScript, runs `oxc_transformer` (TS-only stripping,
/// no JSX transform, no ES downleveling), and re-emits as JavaScript.
fn strip_typescript(
    allocator: &Allocator,
    code: &str,
    generate_sourcemap: bool,
) -> (String, Option<oxc_sourcemap::SourceMap>) {
    let source_type = oxc_span::SourceType::mjs().with_typescript(true);
    let ret = oxc_parser::Parser::new(allocator, code, source_type).parse();

    if !ret.errors.is_empty() {
        // If parsing fails, return the code unchanged — the downstream
        // consumer will report a better error.
        return (code.to_string(), None);
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

    let codegen_options = CodegenOptions {
        single_quote: false,
        source_map_path: if generate_sourcemap {
            Some(std::path::PathBuf::from("intermediate.js"))
        } else {
            None
        },
        ..CodegenOptions::default()
    };
    let result = oxc_codegen::Codegen::new()
        .with_options(codegen_options)
        .build(&program);
    (result.code, result.map)
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

    pub(super) fn compile_astro_with_options(
        source: &str,
        options: TransformOptions,
    ) -> TransformResult {
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

    // === Bug #2 regression: semicolons after return in slot-aware statements ===

    #[test]
    fn test_slot_aware_return_has_semicolon() {
        // A ternary in a component child that routes to named slots triggers
        // print_slot_aware_statement → ReturnStatement. The return must end
        // with a semicolon to avoid ASI hazards.
        let source = r#"---
import Component from "test";
---
<Component>{(() => {
  if (condition) {
    return <div slot="a">A</div>;
  }
  return <div slot="b">B</div>;
})()}</Component>"#;
        let output = compile_astro(source);

        // The compiled output should not contain "return " followed by something
        // without a semicolon before the newline. We check that every "return "
        // in the output has a matching ";" on the same statement.
        assert!(
            !output.contains("return \n"),
            "Return statement should not be followed by bare newline (ASI hazard): {output}"
        );
    }

    // === Bug #3 regression: transition:persist-props on HTML elements ===

    #[test]
    fn test_transition_persist_props_html_element() {
        // transition:persist-props on an HTML element should produce a simple
        // rename to data-astro-transition-persist-props, NOT trigger
        // $$createTransitionScope hash generation in the template body.
        let source = r#"<div transition:persist-props="all">content</div>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("data-astro-transition-persist-props"),
            "Should rename transition:persist-props to data-astro-transition-persist-props: {output}"
        );
        // $$createTransitionScope appears in the import, but should NOT appear
        // in the template body (i.e. inside the $$render`` template literal).
        let template_start = output.find("$$render`").unwrap();
        let template_body = &output[template_start..];
        assert!(
            !template_body.contains("$$createTransitionScope("),
            "transition:persist-props should NOT invoke $$createTransitionScope in template: {output}"
        );
    }

    #[test]
    fn test_transition_persist_and_persist_props_together() {
        // When both transition:persist and transition:persist-props are on the
        // same element, persist should get its normal handling and persist-props
        // should be a simple rename.
        let source = r#"<div transition:persist transition:persist-props="all">content</div>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("data-astro-transition-persist"),
            "Should have data-astro-transition-persist: {output}"
        );
        assert!(
            output.contains("data-astro-transition-persist-props"),
            "Should rename transition:persist-props: {output}"
        );
    }

    #[test]
    fn test_transitions_animation_url_option() {
        // When transitionsAnimationURL is provided, the compiler should use it
        // instead of the default "transitions.css" bare specifier.
        let source = r#"<div transition:persist>content</div>"#;
        let result = compile_astro_with_options(
            source,
            TransformOptions::new()
                .with_internal_url("http://localhost:3000/")
                .with_transitions_animation_url("astro/transitions.css"),
        );

        assert!(
            result.code.contains(r#"import "astro/transitions.css";"#),
            "Should use the provided transitionsAnimationURL: {}",
            result.code
        );
        assert!(
            !result.code.contains(r#"import "transitions.css";"#),
            "Should NOT use the default transitions.css when URL is provided: {}",
            result.code
        );
    }

    #[test]
    fn test_transitions_default_url_without_option() {
        // When transitionsAnimationURL is NOT provided, fall back to "transitions.css".
        let source = r#"<div transition:persist>content</div>"#;
        let output = compile_astro(source);

        assert!(
            output.contains(r#"import "transitions.css";"#),
            "Should use default transitions.css when no URL option is provided: {output}"
        );
    }

    // === CSS scope injection tests ===

    #[test]
    fn test_dynamic_class_with_scoped_styles_no_extra_brace() {
        // When a dynamic class expression is used on an element with scoped styles,
        // the output should not have an extra closing brace leaking into HTML.
        // Regression: `class}=""` was produced due to `}}}}` in format string.
        let source = r#"---
const myClass = "foo";
---
<svg class={myClass}><path d="M0 0"/></svg>
<style>svg { color: red; }</style>"#;
        let output = compile_astro(source);

        // The template literal expression should end with `)}` not `)}}`
        assert!(
            !output.contains("}}"),
            "Should not have double closing braces in template expression: {output}"
        );
        // And there should be no `class}` in the output
        assert!(
            !output.contains("class}"),
            "Should not have malformed class}} attribute: {output}"
        );
        // The $$addAttribute call for class should be well-formed
        assert!(
            output.contains(r#", "class")"#),
            "Should have well-formed $$addAttribute call for class: {output}"
        );
    }

    #[test]
    fn test_component_static_class_merged_with_scope() {
        // When a component has a static class="foo" and scoped styles are active,
        // the scope class should be merged into the value: "class":"foo astro-HASH"
        // NOT two separate "class" keys (which would lose the first one in JS).
        let source = r#"---
import Comp from './Comp.astro';
---
<Comp class="hide" />
<style>div { color: red; }</style>"#;
        let output = compile_astro(source);

        // Should have merged class value like "hide astro-XXXX"
        assert!(
            output.contains(r#""hide astro-"#),
            "Component static class should be merged with scope class: {output}"
        );
        // Should NOT have two separate "class" keys
        let class_count = output.matches(r#""class""#).count();
        assert_eq!(
            class_count, 1,
            "Should have exactly one \"class\" key, got {class_count}: {output}"
        );
    }

    #[test]
    fn test_component_dynamic_class_merged_with_scope() {
        // When a component has a dynamic class={expr} and scoped styles are active,
        // the scope class should be merged: "class":(((expr) ?? "") + " astro-HASH")
        let source = r#"---
import Comp from './Comp.astro';
const cls = "foo";
---
<Comp class={cls} />
<style>div { color: red; }</style>"#;
        let output = compile_astro(source);

        // Should have the ?? "" + " astro-" pattern
        assert!(
            output.contains(r#"?? "") + " astro-"#),
            "Component dynamic class should use nullish coalescing with scope class: {output}"
        );
        // Should NOT have two separate "class" keys
        let class_count = output.matches(r#""class""#).count();
        assert_eq!(
            class_count, 1,
            "Should have exactly one \"class\" key, got {class_count}: {output}"
        );
    }

    #[test]
    fn test_component_no_class_gets_scope_class() {
        // When a component has no class attribute and scoped styles are active,
        // a separate "class":"astro-HASH" should be added.
        let source = r#"---
import Comp from './Comp.astro';
---
<Comp variant="primary" />
<style>div { color: red; }</style>"#;
        let output = compile_astro(source);

        assert!(
            output.contains(r#""astro-"#),
            "Component without class should get scope class prop: {output}"
        );
        // Should have exactly one "class" key
        let class_count = output.matches(r#""class""#).count();
        assert_eq!(
            class_count, 1,
            "Should have exactly one \"class\" key, got {class_count}: {output}"
        );
    }

    // === Test 3: Adversarial/edge-case input tests ===

    #[test]
    fn test_template_literal_injection_in_text() {
        // Backticks in text content should be escaped since output
        // is inside a template literal.
        let source = r"<div>some `backtick` text</div>";
        let output = compile_astro(source);

        assert!(
            output.contains("\\`backtick\\`"),
            "Backticks in text should be escaped: {output}"
        );
    }

    #[test]
    fn test_empty_frontmatter() {
        let source = "---\n---\n<p>hello</p>";
        let output = compile_astro(source);

        assert!(
            output.contains("<p>hello</p>"),
            "Should render content after empty frontmatter: {output}"
        );
        assert!(
            output.contains("$$createComponent"),
            "Should still create component wrapper: {output}"
        );
    }

    #[test]
    fn test_empty_frontmatter_no_template() {
        // Edge case: empty frontmatter with no template content
        let source = "---\n---\n";
        let output = compile_astro(source);

        assert!(
            output.contains("$$createComponent"),
            "Should still create component wrapper: {output}"
        );
    }

    #[test]
    fn test_deeply_nested_ternary_in_expression() {
        let source = r#"<div>{a ? b ? c ? "deep" : "d" : "e" : "f"}</div>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("deep"),
            "Should handle deeply nested ternary: {output}"
        );
    }

    #[test]
    fn test_html_entity_in_text_content() {
        // HTML entities in text should be decoded when used in expressions
        let source = "<div>&lt;script&gt;</div>";
        let output = compile_astro(source);

        // The text content should appear in the template literal
        assert!(
            output.contains("&lt;script&gt;") || output.contains("<script>"),
            "Should handle HTML entities in text: {output}"
        );
    }

    #[test]
    fn test_attribute_with_special_characters() {
        let source = r#"<div data-value="a&b<c>d&quot;e"></div>"#;
        let output = compile_astro(source);

        // The attribute should be preserved or properly escaped
        assert!(
            output.contains("data-value="),
            "Should include the attribute: {output}"
        );
    }

    // === Test 5: JSX statement coverage (for/while/try/throw in JSX) ===

    #[test]
    fn test_for_statement_in_jsx_expression() {
        let source = r"<div>{(() => {
  const items = [];
  for (let i = 0; i < 3; i++) {
    items.push(i);
  }
  return items;
})()}</div>";
        let output = compile_astro(source);

        assert!(
            output.contains("for"),
            "Should handle for statement in JSX: {output}"
        );
    }

    #[test]
    fn test_while_statement_in_jsx_expression() {
        let source = r"<div>{(() => {
  let i = 0;
  while (i < 3) {
    i++;
  }
  return i;
})()}</div>";
        let output = compile_astro(source);

        assert!(
            output.contains("while"),
            "Should handle while statement in JSX: {output}"
        );
    }

    #[test]
    fn test_try_catch_in_jsx_expression() {
        let source = r#"<div>{(() => {
  try {
    return riskyCall();
  } catch (e) {
    return "fallback";
  }
})()}</div>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("try") && output.contains("catch"),
            "Should handle try/catch in JSX: {output}"
        );
    }

    #[test]
    fn test_throw_in_jsx_expression() {
        let source = r#"<div>{(() => {
  if (!data) {
    throw new Error("missing");
  }
  return data;
})()}</div>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("throw"),
            "Should handle throw in JSX: {output}"
        );
    }

    // === Test 7: Empty frontmatter variations ===

    #[test]
    fn test_frontmatter_only_comments() {
        let source = "---\n// just a comment\n---\n<p>hi</p>";
        let output = compile_astro(source);

        assert!(
            output.contains("<p>hi</p>"),
            "Should handle comment-only frontmatter: {output}"
        );
    }

    #[test]
    fn test_no_frontmatter_at_all() {
        let source = "<p>no frontmatter</p>";
        let output = compile_astro(source);

        assert!(
            output.contains("<p>no frontmatter</p>"),
            "Should handle missing frontmatter: {output}"
        );
        assert!(
            output.contains("$$createComponent"),
            "Should still create component: {output}"
        );
    }

    // === Spread attributes ===

    #[test]
    fn test_spread_attributes_on_element() {
        let source = r#"---
const props = { class: "foo", id: "bar" };
---
<div {...props}>content</div>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("$$spreadAttributes"),
            "Should use $$spreadAttributes for spread: {output}"
        );
    }

    // === Boolean and valueless attributes ===

    #[test]
    fn test_boolean_attribute() {
        let source = r"<input disabled />";
        let output = compile_astro(source);

        assert!(
            output.contains("disabled"),
            "Should handle boolean attribute: {output}"
        );
    }

    // === set:html and set:text directives ===

    #[test]
    fn test_set_html_directive() {
        let source = r#"---
const html = "<strong>bold</strong>";
---
<div set:html={html} />"#;
        let output = compile_astro(source);

        assert!(
            output.contains("$$unescapeHTML"),
            "Should use $$unescapeHTML for set:html: {output}"
        );
        // set:html should not appear as an attribute
        assert!(
            !output.contains("set:html="),
            "set:html should be stripped from attributes: {output}"
        );
    }

    #[test]
    fn test_set_text_directive() {
        let source = r#"---
const text = "hello <world>";
---
<div set:text={text} />"#;
        let output = compile_astro(source);

        // set:text should not appear as an attribute
        assert!(
            !output.contains("set:text="),
            "set:text should be stripped from attributes: {output}"
        );
    }

    // === Conditional rendering patterns ===

    #[test]
    fn test_logical_and_rendering() {
        let source = r"<div>{show && <p>visible</p>}</div>";
        let output = compile_astro(source);

        assert!(
            output.contains("show") && output.contains("<p>visible</p>"),
            "Should handle logical AND rendering: {output}"
        );
    }

    #[test]
    fn test_ternary_rendering() {
        let source = r"<div>{show ? <p>yes</p> : <p>no</p>}</div>";
        let output = compile_astro(source);

        assert!(
            output.contains("<p>yes</p>") && output.contains("<p>no</p>"),
            "Should handle ternary rendering: {output}"
        );
    }

    // === Map/iteration patterns ===

    #[test]
    fn test_array_map_rendering() {
        let source = r#"---
const items = ["a", "b", "c"];
---
<ul>{items.map(item => <li>{item}</li>)}</ul>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("<li>"),
            "Should handle array map rendering: {output}"
        );
    }

    // === Transition directives ===

    #[test]
    fn test_transition_name_on_element() {
        let source = r#"<div transition:name="fade">content</div>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("$$renderTransition") || output.contains("data-astro-transition-scope"),
            "Should handle transition:name: {output}"
        );
        assert!(
            output.contains("fade"),
            "Should include transition name: {output}"
        );
    }

    #[test]
    fn test_transition_persist_on_element() {
        let source = r"<div transition:persist>content</div>";
        let output = compile_astro(source);

        assert!(
            output.contains("data-astro-transition-persist")
                || output.contains("$$createTransitionScope"),
            "Should handle transition:persist: {output}"
        );
    }

    // === Test 4: Slot analysis tests (through full compilation) ===

    #[test]
    fn test_slot_ternary_multiple_named_slots() {
        // Ternary expression with different slot names on each branch
        // should trigger $$mergeSlots.
        let source = r#"---
import Component from "test";
---
<Component>{cond ? <div slot="a">A</div> : <div slot="b">B</div>}</Component>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("$$mergeSlots"),
            "Ternary with different slot names should use $$mergeSlots: {output}"
        );
    }

    #[test]
    fn test_slot_ternary_same_slot_name() {
        // Ternary where both branches have the SAME slot name
        // should not need $$mergeSlots — it's a single slot.
        let source = r#"---
import Component from "test";
---
<Component>{cond ? <div slot="x">A</div> : <span slot="x">B</span>}</Component>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("\"x\":"),
            "Both branches with same slot name should produce slot 'x': {output}"
        );
    }

    #[test]
    fn test_slot_logical_and_named() {
        // Logical AND with a named slot
        let source = r#"---
import Component from "test";
---
<Component>{show && <div slot="footer">Footer</div>}</Component>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("\"footer\":"),
            "Logical AND with named slot should produce slot: {output}"
        );
    }

    #[test]
    fn test_slot_default_and_named_together() {
        // Mix of default and named slot children
        let source = r#"---
import Component from "test";
---
<Component>
    <p>Default content</p>
    <div slot="header">Header</div>
</Component>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("\"default\":"),
            "Should have default slot: {output}"
        );
        assert!(
            output.contains("\"header\":"),
            "Should have header slot: {output}"
        );
    }

    #[test]
    fn test_slot_no_children() {
        // Component with no children — no slots at all
        let source = r#"---
import Component from "test";
---
<Component />"#;
        let output = compile_astro(source);

        // Should not have any slot definitions
        assert!(
            !output.contains("\"default\":") || output.contains("\"default\": () =>"),
            "Self-closing component should have no slots: {output}"
        );
    }

    #[test]
    fn test_slot_dynamic_slot_attribute() {
        // Dynamic slot name: slot={name}
        let source = r#"---
import Component from "test";
const slotName = "dynamic";
---
<Component><div slot={slotName}>Content</div></Component>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("slotName"),
            "Dynamic slot name should reference the variable: {output}"
        );
    }

    #[test]
    fn test_slot_fragment_children() {
        // Fragment as component child
        let source = r#"---
import Component from "test";
---
<Component><>fragment child</></Component>"#;
        let output = compile_astro(source);

        assert!(
            output.contains("fragment child"),
            "Fragment children should be rendered: {output}"
        );
    }

    // === Test 6: JSXAttributeValue::Element/Fragment paths ===

    #[test]
    fn test_jsx_element_as_attribute_value_on_component() {
        // JSX element as attribute value on a component renders as "[JSX]"
        let source = r#"---
import Comp from "test";
---
<Comp attr=<span>hi</span> />"#;
        let output = compile_astro(source);

        assert!(
            output.contains("[JSX]"),
            "JSX element as component attribute should render as [JSX]: {output}"
        );
    }

    #[test]
    fn test_jsx_fragment_as_attribute_value_on_component() {
        // JSX fragment as attribute value on a component renders as "[Fragment]"
        let source = r#"---
import Comp from "test";
---
<Comp attr=<>fragment</> />"#;
        let output = compile_astro(source);

        assert!(
            output.contains("[Fragment]"),
            "JSX fragment as component attribute should render as [Fragment]: {output}"
        );
    }

    // === Test 8: Dynamic slot names on <slot> element ===

    #[test]
    fn test_slot_element_with_static_name() {
        // <slot name="header" /> should generate a renderSlot call with "header"
        let source = r#"<slot name="header" />"#;
        let output = compile_astro(source);

        assert!(
            output.contains("$$renderSlot") && output.contains("header"),
            "Static <slot name> should use $$renderSlot with 'header': {output}"
        );
    }

    #[test]
    fn test_slot_element_default() {
        // <slot /> without name should use "default"
        let source = "<slot />";
        let output = compile_astro(source);

        assert!(
            output.contains("$$renderSlot") && output.contains("default"),
            "Default <slot /> should use $$renderSlot with 'default': {output}"
        );
    }

    #[test]
    fn test_slot_element_with_dynamic_name() {
        // <slot name={expr} /> should generate a dynamic slot name
        let source = r#"---
const slotName = "dynamic";
---
<slot name={slotName} />"#;
        let output = compile_astro(source);

        assert!(
            output.contains("$$renderSlot"),
            "Dynamic <slot name={{expr}}> should use $$renderSlot: {output}"
        );
        assert!(
            output.contains("slotName"),
            "Dynamic slot name should reference the variable: {output}"
        );
    }

    #[test]
    fn test_slot_element_with_fallback_content() {
        // <slot>fallback</slot> should include fallback content
        let source = "<slot><p>Fallback content</p></slot>";
        let output = compile_astro(source);

        assert!(
            output.contains("$$renderSlot"),
            "Slot with fallback should use $$renderSlot: {output}"
        );
        assert!(
            output.contains("Fallback content"),
            "Fallback content should be present: {output}"
        );
    }
}
