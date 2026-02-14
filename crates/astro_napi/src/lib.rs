//! Astro file compilation to JavaScript.

mod error;

#[cfg(all(
    feature = "allocator",
    not(any(
        target_arch = "arm",
        target_os = "freebsd",
        target_os = "windows",
        target_family = "wasm"
    ))
))]
#[global_allocator]
static ALLOC: mimalloc_safe::MiMalloc = mimalloc_safe::MiMalloc;

use std::mem;

use napi::{bindgen_prelude::AsyncTask, Task};
use napi_derive::napi;

use std::collections::HashMap;

use crate::error::CompilerError;
use astro_codegen::{extract_styles, transform, HoistedScriptType, TransformOptions};
use oxc_allocator::Allocator;
use oxc_estree::CompactTSSerializer;
use oxc_estree::ESTree;
use oxc_parser::{ParseOptions, Parser};
use oxc_span::SourceType;

/// Controls whether and how source maps are emitted.
#[napi(string_enum)]
pub enum SourcemapOption {
    /// Generate a source map in the `map` field of the result.
    #[napi(value = "external")]
    External,
    /// Append an inline `//# sourceMappingURL=data:...` comment to the code.
    /// The `map` field will be empty.
    #[napi(value = "inline")]
    Inline,
    /// Both: append the inline comment **and** populate the `map` field.
    #[napi(value = "both")]
    Both,
}

/// Strategy for CSS scoping.
///
/// Determines how Astro scopes CSS selectors to components.
#[napi(string_enum)]
pub enum ScopedStyleStrategy {
    /// Use `:where(.astro-XXXX)` selector (default).
    #[napi(value = "where")]
    Where,
    /// Use `.astro-XXXX` class selector.
    #[napi(value = "class")]
    Class,
    /// Use `[data-astro-cid-XXXX]` attribute selector.
    #[napi(value = "attribute")]
    Attribute,
}

/// Options for compiling Astro files to JavaScript.
#[napi(object)]
#[derive(Default)]
pub struct CompileOptions {
    /// The filename of the Astro component being compiled.
    /// Used in the `$$createComponent` call for debugging.
    pub filename: Option<String>,

    /// A normalized version of the filename used for scope hash generation.
    /// If not provided, falls back to `filename`.
    pub normalized_filename: Option<String>,

    /// The import specifier for Astro runtime functions.
    /// Defaults to `"astro/runtime/server/index.js"`.
    #[napi(js_name = "internalURL")]
    pub internal_url: Option<String>,

    /// Source map generation mode.
    ///
    /// - `"external"`: populate the `map` field with a JSON source map.
    /// - `"inline"`: append an inline `//# sourceMappingURL=data:...` comment; `map` will be empty.
    /// - `"both"`: append the inline comment **and** populate `map`.
    /// - `undefined`: no source map (default).
    #[napi(ts_type = "'external' | 'inline' | 'both'")]
    pub sourcemap: Option<SourcemapOption>,

    /// Arguments passed to `$$createAstro` when the Astro global is used.
    /// Defaults to `"https://astro.build"`.
    pub astro_global_args: Option<String>,

    /// Whether to collapse whitespace in the HTML output.
    /// **Stub**: not yet implemented.
    ///
    /// @default false
    pub compact: Option<bool>,

    /// Enable scoped slot result handling.
    /// When `true`, slot callbacks receive the `$$result` render context parameter.
    ///
    /// @default false
    pub result_scoped_slot: Option<bool>,

    /// Strategy for CSS scoping.
    ///
    /// @default "where"
    #[napi(ts_type = "'where' | 'class' | 'attribute'")]
    pub scoped_style_strategy: Option<ScopedStyleStrategy>,

    /// URL for the view transitions animation CSS.
    /// When set, replaces the default `"transitions.css"` bare specifier in the emitted import.
    #[napi(js_name = "transitionsAnimationURL")]
    pub transitions_animation_url: Option<String>,

    /// Whether to annotate generated code with the source file path.
    /// **Stub**: not yet implemented.
    ///
    /// @default false
    pub annotate_source_file: Option<bool>,

    /// Whether to strip HTML comments from component slot children.
    /// Matches the official Astro compiler behavior by default.
    ///
    /// @default true
    pub strip_slot_comments: Option<bool>,

    /// Whether the caller has a `resolvePath` function.
    ///
    /// When `true`, the codegen will:
    /// - Skip emitting `$$createMetadata` import
    /// - Skip emitting `import * as $$moduleN` re-imports
    /// - Skip emitting `export const $$metadata = ...`
    /// - Use plain string literals instead of `$$metadata.resolvePath(...)`
    ///
    /// The actual path resolution is done by the JS wrapper layer using
    /// the `resolvePath` callback post-compilation.
    ///
    /// @default false
    pub resolve_path_provided: Option<bool>,

    /// Preprocessed style content, indexed by extractable style order.
    ///
    /// When provided, the codegen uses these strings as CSS content instead
    /// of reading from the AST's `<style>` text children. Each entry
    /// corresponds to an extractable style in document order (matching the
    /// indices from `extractStylesSync`).
    ///
    /// An entry of `undefined` means "use the original content from the AST".
    /// An entry of `""` means "style had a preprocessing error — use empty content".
    pub preprocessed_styles: Option<Vec<Option<String>>>,
}

/// A hoisted script extracted from an Astro component.
#[napi(object)]
#[derive(Clone)]
pub struct HoistedScript {
    /// The script type: `"inline"` or `"external"`.
    #[napi(js_name = "type")]
    pub script_type: String,
    /// The inline script code (when type is `"inline"`).
    pub code: Option<String>,
    /// The external script src URL (when type is `"external"`).
    pub src: Option<String>,
}

/// A component reference found in the template (hydrated, client-only, or server-deferred).
#[napi(object)]
#[derive(Clone)]
pub struct Component {
    /// The export name from the module (e.g., `"default"`).
    pub export_name: String,
    /// The local variable name used in the component.
    pub local_name: String,
    /// The import specifier (e.g., `"../components/Counter.jsx"`).
    pub specifier: String,
    /// The resolved path (empty string if unresolved).
    pub resolved_path: String,
}

/// Result of compiling an Astro file.
#[napi(object)]
pub struct CompileResult {
    /// The generated JavaScript code.
    pub code: String,
    /// Source map JSON string. Contains a JSON-encoded source map when
    /// `sourcemap: true` was passed in options. Empty string otherwise.
    pub map: String,
    /// CSS scope hash for the component.
    pub scope: String,
    /// Extracted CSS from `<style>` tags.
    pub css: Vec<String>,
    /// Hoisted scripts extracted from the template.
    pub scripts: Vec<HoistedScript>,
    /// Components with `client:*` hydration directives (except `client:only`).
    pub hydrated_components: Vec<Component>,
    /// Components with `client:only` directive.
    pub client_only_components: Vec<Component>,
    /// Components with `server:defer` directive.
    pub server_components: Vec<Component>,
    /// Whether the template contains an explicit `<head>` element.
    pub contains_head: bool,
    /// Whether the component propagates head content.
    pub propagation: bool,
    /// Style processing errors.
    pub style_error: Vec<String>,
    /// Diagnostic messages.
    pub diagnostics: Vec<String>,
    /// Any compilation errors encountered.
    pub errors: Vec<CompilerError>,
}

/// An extractable `<style>` block from an Astro component.
///
/// Returned by `extractStylesSync` for each `<style>` element that would be
/// extracted and processed during compilation.
#[napi(object)]
#[derive(Clone)]
pub struct StyleBlock {
    /// Zero-based index of this style block among all extractable styles.
    pub index: u32,
    /// The CSS/preprocessor text content between `<style>` and `</style>`.
    pub content: String,
    /// The element's attributes as key-value pairs.
    /// Only quoted and empty (boolean) attributes are included — expression
    /// attributes (like `define:vars={...}`) are omitted.
    pub attrs: HashMap<String, String>,
}

/// Extract style block metadata from an Astro source without performing compilation.
///
/// Returns an array of style blocks in document order. Each block contains the
/// text content and attributes of an extractable `<style>` element.
///
/// This is the first step in the "Rust extract → TS preprocess → Rust compile"
/// pipeline for `preprocessStyle` support.
#[napi]
pub fn extract_styles_sync(source_text: String) -> Vec<StyleBlock> {
    let allocator = Allocator::default();
    let source_type = SourceType::astro();

    let ret = Parser::new(&allocator, &source_text, source_type)
        .with_options(ParseOptions::default())
        .parse_astro();

    if !ret.errors.is_empty() {
        return Vec::new();
    }

    let blocks = extract_styles(&ret.root);
    blocks
        .into_iter()
        .map(|b| StyleBlock {
            index: b.index as u32,
            content: b.content,
            attrs: b.attrs.into_iter().collect(),
        })
        .collect()
}

fn napi_to_codegen_sourcemap(s: &Option<SourcemapOption>) -> astro_codegen::SourcemapOption {
    match s {
        Some(SourcemapOption::External) => astro_codegen::SourcemapOption::External,
        Some(SourcemapOption::Inline) => astro_codegen::SourcemapOption::Inline,
        Some(SourcemapOption::Both) => astro_codegen::SourcemapOption::Both,
        None => astro_codegen::SourcemapOption::None,
    }
}

fn napi_to_codegen_strategy(s: &Option<ScopedStyleStrategy>) -> astro_codegen::ScopedStyleStrategy {
    match s {
        Some(ScopedStyleStrategy::Class) => astro_codegen::ScopedStyleStrategy::Class,
        Some(ScopedStyleStrategy::Attribute) => astro_codegen::ScopedStyleStrategy::Attribute,
        _ => astro_codegen::ScopedStyleStrategy::Where,
    }
}

fn compile_astro_impl(source_text: &str, options: &CompileOptions) -> CompileResult {
    let allocator = Allocator::default();
    let source_type = SourceType::astro();

    // Parse the Astro file
    let ret = Parser::new(&allocator, source_text, source_type)
        .with_options(ParseOptions::default())
        .parse_astro();

    // If there are parse errors, return them
    if !ret.errors.is_empty() {
        let errors = CompilerError::from_diagnostics("", source_text, ret.errors);
        return CompileResult {
            code: String::new(),
            map: String::new(),
            scope: String::new(),
            css: Vec::new(),
            scripts: Vec::new(),
            hydrated_components: Vec::new(),
            client_only_components: Vec::new(),
            server_components: Vec::new(),
            contains_head: false,
            propagation: false,
            style_error: Vec::new(),
            diagnostics: Vec::new(),
            errors,
        };
    }

    // Build codegen options
    let resolve_path_provided = options.resolve_path_provided.unwrap_or(false);

    let codegen_options = TransformOptions {
        filename: options.filename.clone(),
        normalized_filename: options.normalized_filename.clone(),
        internal_url: options.internal_url.clone(),
        sourcemap: napi_to_codegen_sourcemap(&options.sourcemap),
        astro_global_args: options.astro_global_args.clone(),
        compact: options.compact.unwrap_or(false),
        result_scoped_slot: options.result_scoped_slot.unwrap_or(false),
        scoped_style_strategy: napi_to_codegen_strategy(&options.scoped_style_strategy),
        transitions_animation_url: options.transitions_animation_url.clone(),
        annotate_source_file: options.annotate_source_file.unwrap_or(false),
        strip_slot_comments: options.strip_slot_comments.unwrap_or(true),
        resolve_path: None,
        resolve_path_provided,
        preprocessed_styles: options.preprocessed_styles.clone(),
    };

    // Generate JavaScript code
    let result = transform(&allocator, source_text, codegen_options, &ret.root);

    // Convert internal types to NAPI types
    let scripts = result
        .scripts
        .into_iter()
        .map(|s| match s.script_type {
            HoistedScriptType::External => HoistedScript {
                script_type: "external".to_string(),
                src: s.src,
                code: None,
            },
            HoistedScriptType::Inline => HoistedScript {
                script_type: "inline".to_string(),
                code: s.code,
                src: None,
            },
        })
        .collect();

    let map_component = |c: astro_codegen::TransformResultHydratedComponent| Component {
        export_name: c.export_name,
        local_name: c.local_name,
        specifier: c.specifier,
        resolved_path: c.resolved_path,
    };

    let hydrated_components = result
        .hydrated_components
        .into_iter()
        .map(map_component)
        .collect();
    let client_only_components = result
        .client_only_components
        .into_iter()
        .map(map_component)
        .collect();
    let server_components = result
        .server_components
        .into_iter()
        .map(map_component)
        .collect();

    CompileResult {
        code: result.code,
        map: result.map,
        scope: result.scope,
        css: result.css,
        scripts,
        hydrated_components,
        client_only_components,
        server_components,
        contains_head: result.contains_head,
        propagation: result.propagation,
        style_error: result.style_error,
        diagnostics: result.diagnostics,
        errors: Vec::new(),
    }
}

/// Compile Astro file to JavaScript synchronously on current thread.
///
/// @example
/// ```javascript
/// import { compileAstroSync } from '@astrojs/compiler-binding';
///
/// const result = compileAstroSync(`---
/// const name = "World";
/// ---
/// <h1>Hello {name}!</h1>`, {
///   filename: 'Component.astro',
/// });
///
/// console.log(result.code); // Generated JavaScript
/// ```
#[napi]
pub fn compile_astro_sync(source_text: String, options: Option<CompileOptions>) -> CompileResult {
    let options = options.unwrap_or_default();
    compile_astro_impl(&source_text, &options)
}

pub struct CompileTask {
    source_text: String,
    options: CompileOptions,
}

#[napi]
impl Task for CompileTask {
    type JsValue = CompileResult;
    type Output = CompileResult;

    fn compute(&mut self) -> napi::Result<Self::Output> {
        let source_text = mem::take(&mut self.source_text);
        Ok(compile_astro_impl(&source_text, &self.options))
    }

    fn resolve(&mut self, _: napi::Env, result: Self::Output) -> napi::Result<Self::JsValue> {
        Ok(result)
    }
}

/// Compile Astro file to JavaScript asynchronously on a separate thread.
///
/// Generally `compileAstroSync` is preferable to use as it does not have the overhead
/// of spawning a thread. If you need to parallelize compilation of multiple files,
/// it is recommended to use worker threads.
///
/// @example
/// ```javascript
/// import { compileAstro } from '@astrojs/compiler-binding';
///
/// const result = await compileAstro(`---
/// const name = "World";
/// ---
/// <h1>Hello {name}!</h1>`, {
///   filename: 'Component.astro',
/// });
///
/// console.log(result.code); // Generated JavaScript
/// ```
#[napi]
pub fn compile_astro(
    source_text: String,
    options: Option<CompileOptions>,
) -> AsyncTask<CompileTask> {
    let options = options.unwrap_or_default();
    AsyncTask::new(CompileTask {
        source_text,
        options,
    })
}

/// Result of parsing an Astro file into an AST.
#[napi(object)]
pub struct ParseResult {
    /// The AST serialized as a JSON string (ESTree-compatible format from oxc).
    /// Call `JSON.parse()` on this to get the AST object.
    pub ast: String,
    /// Any parse errors encountered.
    pub errors: Vec<CompilerError>,
}

fn parse_astro_impl(source_text: &str) -> ParseResult {
    let allocator = Allocator::default();
    let source_type = SourceType::astro();

    let ret = Parser::new(&allocator, source_text, source_type)
        .with_options(ParseOptions::default())
        .parse_astro();

    let errors = if ret.errors.is_empty() {
        Vec::new()
    } else {
        CompilerError::from_diagnostics("", source_text, ret.errors)
    };

    // Serialize the AST to JSON using the ESTree serializer
    let mut serializer = CompactTSSerializer::new(false);
    ret.root.serialize(&mut serializer);
    let ast = serializer.into_string();

    ParseResult { ast, errors }
}

/// Parse an Astro file into an AST synchronously.
///
/// Returns the oxc AST in ESTree-compatible JSON format.
///
/// @example
/// ```javascript
/// import { parseAstroSync } from '@astrojs/compiler-binding';
///
/// const { ast } = parseAstroSync(`---
/// const name = "World";
/// ---
/// <h1>Hello {name}!</h1>`);
///
/// const tree = JSON.parse(ast);
/// console.log(tree.type); // "AstroRoot"
/// ```
#[napi]
pub fn parse_astro_sync(source_text: String) -> ParseResult {
    parse_astro_impl(&source_text)
}

pub struct ParseTask {
    source_text: String,
}

#[napi]
impl Task for ParseTask {
    type JsValue = ParseResult;
    type Output = ParseResult;

    fn compute(&mut self) -> napi::Result<Self::Output> {
        let source_text = mem::take(&mut self.source_text);
        Ok(parse_astro_impl(&source_text))
    }

    fn resolve(&mut self, _: napi::Env, result: Self::Output) -> napi::Result<Self::JsValue> {
        Ok(result)
    }
}

/// Parse an Astro file into an AST asynchronously on a separate thread.
///
/// Returns the oxc AST in ESTree-compatible JSON format.
#[napi]
pub fn parse_astro(source_text: String) -> AsyncTask<ParseTask> {
    AsyncTask::new(ParseTask { source_text })
}
