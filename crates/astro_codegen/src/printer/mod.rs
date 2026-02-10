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

use oxc_codegen::CodegenOptions;
use oxc_span::{GetSpan, Span};

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
    /// Sourcemap builder (present when `options.sourcemap` is true)
    sourcemap_builder: Option<AstroSourcemapBuilder>,
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

        // Initialize sourcemap builder if requested
        let sourcemap_builder = if options.sourcemap {
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
    #[expect(clippy::items_after_statements)]
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
        let generate_sourcemap = self.options.sourcemap;

        // Strip TypeScript from the generated code, optionally producing a sourcemap.
        let (code, map) = strip_typescript(self.allocator, &intermediate_code, generate_sourcemap);

        // Compose sourcemaps if both phases produced one.
        //
        // Phase 1 maps intermediate positions → original .astro positions.
        // Phase 2 maps final positions → intermediate positions.
        // Composition gives us final positions → original .astro positions.
        //
        // However, Phase 2 (oxc_codegen re-emit) only produces tokens at AST
        // node boundaries.  The template literal `$$render\`...\`` is one AST
        // node, so all the fine-grained Phase 1 tokens *inside* it are lost
        // during naïve composition.
        //
        // Fix: after composing the two maps, carry forward any Phase 1 tokens
        // that were not covered by Phase 2 by computing the line-offset
        // between the intermediate and final code and adjusting their
        // generated positions.
        // Token struct used to collect, sort, and deduplicate all
        // mappings before feeding them to the builder (which requires
        // tokens in ascending generated-position order).
        struct RawToken {
            dst_line: u32,
            dst_col: u32,
            src_line: u32,
            src_col: u32,
            name: Option<String>,
        }

        let map = if let (Some(phase2_map), Some(phase1_sm)) = (map, phase1_sourcemap) {
            let phase1_map = phase1_sm.into_sourcemap();
            let source_path = self.options.filename.as_deref().unwrap_or("<stdin>");
            let composed = sourcemap_builder::remap_sourcemap(
                &phase2_map,
                &phase1_map,
                source_path,
                self.source_text,
            );

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
                let line_col_info: rustc_hash::FxHashMap<usize, (i64, Vec<usize>)> = (i_line
                    ..inter_lines.len())
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
                let composed_positions: rustc_hash::FxHashSet<(u32, u32)> = composed
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
                let phase2_anchors: rustc_hash::FxHashMap<u32, Vec<(u32, u32, u32)>> = {
                    let mut map: rustc_hash::FxHashMap<u32, Vec<(u32, u32, u32)>> =
                        rustc_hash::FxHashMap::default();
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
                for t in phase1_map.get_tokens() {
                    let gen_line = t.get_dst_line() as usize;
                    // Only consider tokens at or after the template start.
                    if gen_line < i_line {
                        continue;
                    }

                    let token_col = t.get_dst_col();

                    // Try the matched-line path first (exact, leading-ws,
                    // or escape-normalized match).  If absent, fall through
                    // to anchor-based supplementing for DIFFER lines.
                    let (adjusted_line, adjusted_col) =
                        if let Some((ws_delta, escape_positions)) = line_col_info.get(&gen_line) {
                            // gen_line >= i_line (checked above), so this
                            // never underflows.
                            let al = u32::try_from(gen_line - i_line + f_line)
                                .expect("adjusted line exceeds u32");

                            // Count how many escape insertions occur at or
                            // before this token's column.  Each `<\/` escape
                            // adds 1 byte (the `\`) that shifts subsequent
                            // columns.
                            let tc = token_col as usize;
                            let escape_shift = i64::try_from(
                                escape_positions.iter().filter(|&&pos| pos <= tc).count(),
                            )
                            .expect("escape count exceeds i64");

                            let ac = u32::try_from(
                                (i64::from(token_col) + ws_delta + escape_shift).max(0),
                            )
                            .expect("adjusted column exceeds u32");
                            (al, ac)
                        } else {
                            // DIFFER line — try anchor-based supplementing.
                            //
                            // Phase 2 reformats JS expressions inside ${}
                            // (adds spaces, parens, etc.) but leaves template
                            // quasi text identical.  Phase-1 tokens inside
                            // quasi regions can be carried forward by using
                            // the nearest Phase-2 token as a column anchor
                            // and verifying the text matches at the computed
                            // final position.
                            let Ok(gen_line_u32) = u32::try_from(gen_line) else {
                                continue;
                            };

                            if let Some(anchors) = phase2_anchors.get(&gen_line_u32) {
                                // Find the nearest anchor with inter_col <=
                                // token_col (the last one in sorted order that
                                // doesn't exceed it).
                                let nearest =
                                    anchors.iter().rev().find(|&&(ic, _, _)| ic <= token_col);
                                if let Some(&(anchor_ic, anchor_fl, anchor_fc)) = nearest {
                                    let delta = i64::from(anchor_fc) - i64::from(anchor_ic);
                                    let candidate_col = u32::try_from(
                                        (i64::from(token_col) + delta).max(0),
                                    )
                                    .expect("candidate column exceeds u32");

                                    // Verify: the text at the candidate final
                                    // position must match the intermediate text at
                                    // the token position.  This confirms we are in
                                    // a quasi region (not a reformatted expression).
                                    let i_text =
                                        inter_lines.get(gen_line).copied().unwrap_or("");
                                    let f_text = final_lines
                                        .get(anchor_fl as usize)
                                        .copied()
                                        .unwrap_or("");
                                    let tc = token_col as usize;
                                    let cc = candidate_col as usize;
                                    // Use a short verification window; 2 bytes is
                                    // enough to confirm we're at the same quasi
                                    // text (e.g. "<p", "</", etc.).
                                    let verify_len = 2;
                                    let text_matches = tc + verify_len <= i_text.len()
                                        && cc + verify_len <= f_text.len()
                                        && i_text.as_bytes()[tc..tc + verify_len]
                                            == f_text.as_bytes()[cc..cc + verify_len];

                                    if text_matches {
                                        (anchor_fl, candidate_col)
                                    } else {
                                        continue;
                                    }
                                } else {
                                    continue;
                                }
                            } else {
                                // No Phase-2 anchors for this intermediate line.
                                //
                                // This happens for lines that are pure template
                                // literal quasi text (no JS expressions).  Phase-2
                                // reformatting may shift these lines by inserting
                                // extra lines for object literal properties, but
                                // the text content remains identical.
                                //
                                // Search a window of final lines around the
                                // expected position for a line containing the same
                                // text at the token's column.
                                let i_text =
                                    inter_lines.get(gen_line).copied().unwrap_or("");
                                let tc = token_col as usize;
                                let verify_len = 2;
                                if tc + verify_len > i_text.len() {
                                    continue;
                                }
                                let needle = &i_text.as_bytes()[tc..tc + verify_len];

                                // Expected final line based on the template offset.
                                // gen_line >= i_line (checked at loop top).
                                let expected_fl = gen_line - i_line + f_line;
                                // Search ±5 lines around expected position to
                                // account for Phase-2 reformatting inserting a
                                // few extra lines.
                                let search_start = expected_fl.saturating_sub(5);
                                let search_end =
                                    (expected_fl + 6).min(final_lines.len());

                                let mut found = None;
                                for (fl, f_text) in final_lines
                                    .iter()
                                    .enumerate()
                                    .take(search_end)
                                    .skip(search_start)
                                {
                                    // Check same column (the text is quasi literal,
                                    // so the column should be identical).
                                    if tc + verify_len <= f_text.len()
                                        && &f_text.as_bytes()[tc..tc + verify_len]
                                            == needle
                                    {
                                        let fl_u32 = u32::try_from(fl)
                                            .expect("final line exceeds u32");
                                        found = Some((fl_u32, token_col));
                                        break;
                                    }
                                    // Also check with leading whitespace adjustment:
                                    // Phase-2 may have added or removed leading ws.
                                    let f_trimmed = f_text.trim_start();
                                    let i_trimmed = i_text.trim_start();
                                    let f_ws =
                                        i64::try_from(f_text.len() - f_trimmed.len())
                                            .unwrap_or(0);
                                    let i_ws =
                                        i64::try_from(i_text.len() - i_trimmed.len())
                                            .unwrap_or(0);
                                    let ws_adj = f_ws - i_ws;
                                    let adj_col_i64 =
                                        (i64::from(token_col) + ws_adj).max(0);
                                    let adj_col = usize::try_from(adj_col_i64)
                                        .expect("adjusted column exceeds usize");
                                    if adj_col + verify_len <= f_text.len()
                                        && &f_text.as_bytes()
                                            [adj_col..adj_col + verify_len]
                                            == needle
                                    {
                                        let fl_u32 = u32::try_from(fl)
                                            .expect("final line exceeds u32");
                                        let ac = u32::try_from(adj_col)
                                            .expect("adjusted col exceeds u32");
                                        found = Some((fl_u32, ac));
                                        break;
                                    }
                                }

                                let Some((fl, fc)) = found else {
                                    continue;
                                };
                                (fl, fc)
                            }
                        };

                    // Skip if already covered by composition.
                    if composed_positions.contains(&(adjusted_line, adjusted_col)) {
                        continue;
                    }

                    // Skip if beyond final code.
                    if (adjusted_line as usize) >= final_lines.len() {
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

                // Sort by generated position (line first, then column).
                all_tokens
                    .sort_by(|a, b| a.dst_line.cmp(&b.dst_line).then(a.dst_col.cmp(&b.dst_col)));

                // Deduplicate tokens at the same generated position.
                all_tokens.dedup_by(|a, b| a.dst_line == b.dst_line && a.dst_col == b.dst_col);

                // Build the final sourcemap in sorted order.
                let mut builder = oxc_sourcemap::SourceMapBuilder::default();
                let src_id = builder.set_source_and_content(source_path, self.source_text);

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

            composed.to_json_string()
        } else {
            String::new()
        };

        TransformResult {
            code,
            map,
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

        // Handle <script> elements that should be hoisted
        if name == "script" && Self::is_hoisted_script(el) {
            if self.element_depth > 0 {
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
}
