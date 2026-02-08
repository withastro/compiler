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

use crate::error::OxcError;
use astro_codegen::{AstroCodegen, HoistedScriptType, TransformOptions};
use oxc_allocator::Allocator;
use oxc_parser::{ParseOptions, Parser};
use oxc_span::SourceType;

/// Options for compiling Astro files to JavaScript.
///
/// Matches the Go compiler's `TransformOptions` from `@astrojs/compiler`.
/// Some fields (such as `sourcemap`, `compact`, CSS scoping) are stubs accepted for API compatibility.
#[napi(object)]
#[derive(Default)]
pub struct AstroCompileOptions {
    /// The filename of the Astro component being compiled.
    /// Used in the `$$createComponent` call for debugging.
    pub filename: Option<String>,

    /// A normalized version of the filename used for scope hash generation.
    /// If not provided, falls back to `filename`.
    pub normalized_filename: Option<String>,

    /// The import specifier for Astro runtime functions.
    /// Defaults to `"astro/runtime/server/index.js"`.
    pub internal_url: Option<String>,

    /// Whether to generate a source map.
    /// **Stub**: not yet implemented.
    ///
    /// @default false
    pub sourcemap: Option<bool>,

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

    /// Strategy for CSS scoping: `"where"`, `"class"`, or `"attribute"`.
    /// **Stub**: not yet implemented.
    ///
    /// @default "where"
    pub scoped_style_strategy: Option<String>,

    /// URL for the view transitions animation CSS.
    /// **Stub**: not yet implemented.
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
}

/// A hoisted script extracted from an Astro component.
#[napi(object)]
pub struct NapiHoistedScript {
    /// The script type: `"inline"` or `"external"`.
    #[napi(js_name = "type")]
    pub script_type: String,
    /// The inline script code (when type is `"inline"`).
    pub code: Option<String>,
    /// The external script src URL (when type is `"external"`).
    pub src: Option<String>,
}

/// A hydrated component reference found in the template.
#[napi(object)]
pub struct NapiHydratedComponent {
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
#[napi]
pub struct AstroCompileResult {
    code: String,
    map: String,
    scope: String,
    css: Vec<String>,
    scripts: Vec<NapiHoistedScript>,
    hydrated_components: Vec<NapiHydratedComponent>,
    client_only_components: Vec<NapiHydratedComponent>,
    server_components: Vec<NapiHydratedComponent>,
    contains_head: bool,
    propagation: bool,
    style_error: Vec<String>,
    diagnostics: Vec<String>,
    errors: Vec<OxcError>,
}

#[napi]
impl AstroCompileResult {
    /// The generated JavaScript code.
    #[napi(getter)]
    pub fn code(&mut self) -> String {
        mem::take(&mut self.code)
    }

    /// Source map JSON string (empty until sourcemap support is implemented).
    #[napi(getter)]
    pub fn map(&mut self) -> String {
        mem::take(&mut self.map)
    }

    /// CSS scope hash for the component.
    #[napi(getter)]
    pub fn scope(&mut self) -> String {
        mem::take(&mut self.scope)
    }

    /// Extracted CSS from `<style>` tags (empty until CSS support).
    #[napi(getter)]
    pub fn css(&mut self) -> Vec<String> {
        mem::take(&mut self.css)
    }

    /// Hoisted scripts extracted from the template.
    #[napi(getter)]
    pub fn scripts(&mut self) -> Vec<NapiHoistedScript> {
        mem::take(&mut self.scripts)
    }

    /// Components with `client:*` hydration directives (except `client:only`).
    #[napi(getter)]
    pub fn hydrated_components(&mut self) -> Vec<NapiHydratedComponent> {
        mem::take(&mut self.hydrated_components)
    }

    /// Components with `client:only` directive.
    #[napi(getter)]
    pub fn client_only_components(&mut self) -> Vec<NapiHydratedComponent> {
        mem::take(&mut self.client_only_components)
    }

    /// Components with `server:defer` directive.
    #[napi(getter)]
    pub fn server_components(&mut self) -> Vec<NapiHydratedComponent> {
        mem::take(&mut self.server_components)
    }

    /// Whether the template contains an explicit `<head>` element.
    #[napi(getter)]
    pub fn contains_head(&self) -> bool {
        self.contains_head
    }

    /// Whether the component propagates head content.
    #[napi(getter)]
    pub fn propagation(&self) -> bool {
        self.propagation
    }

    /// Style processing errors (stub: always empty).
    #[napi(getter)]
    pub fn style_error(&mut self) -> Vec<String> {
        mem::take(&mut self.style_error)
    }

    /// Diagnostic messages (stub: always empty).
    #[napi(getter, js_name = "diagnostics")]
    pub fn diagnostics(&mut self) -> Vec<String> {
        mem::take(&mut self.diagnostics)
    }

    /// Any compilation errors encountered (oxc-specific).
    #[napi(getter)]
    pub fn errors(&mut self) -> Vec<OxcError> {
        mem::take(&mut self.errors)
    }
}

fn parse_scoped_style_strategy(s: Option<&str>) -> astro_codegen::ScopedStyleStrategy {
    match s {
        Some("class") => astro_codegen::ScopedStyleStrategy::Class,
        Some("attribute") => astro_codegen::ScopedStyleStrategy::Attribute,
        _ => astro_codegen::ScopedStyleStrategy::Where,
    }
}

fn compile_astro_impl(source_text: &str, options: &AstroCompileOptions) -> AstroCompileResult {
    let allocator = Allocator::default();
    let source_type = SourceType::astro();

    // Parse the Astro file
    let ret = Parser::new(&allocator, source_text, source_type)
        .with_options(ParseOptions::default())
        .parse_astro();

    // If there are parse errors, return them
    if !ret.errors.is_empty() {
        let errors = OxcError::from_diagnostics("", source_text, ret.errors);
        return AstroCompileResult {
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
        sourcemap: options.sourcemap.unwrap_or(false),
        astro_global_args: options.astro_global_args.clone(),
        compact: options.compact.unwrap_or(false),
        result_scoped_slot: options.result_scoped_slot.unwrap_or(false),
        scoped_style_strategy: parse_scoped_style_strategy(
            options.scoped_style_strategy.as_deref(),
        ),
        transitions_animation_url: options.transitions_animation_url.clone(),
        annotate_source_file: options.annotate_source_file.unwrap_or(false),
        strip_slot_comments: options.strip_slot_comments.unwrap_or(true),
        resolve_path: None,
        // Signal that caller has resolvePath — codegen skips $$metadata emission,
        // but still uses filepath.Join fallback for resolved_path. The real
        // resolution happens post-compilation in the TS wrapper layer.
        resolve_path_provided,
    };

    // Generate JavaScript code
    let codegen = AstroCodegen::new(&allocator, source_text, codegen_options);
    let result = codegen.build(&ret.root);

    // Convert internal types to NAPI types
    let scripts = result
        .scripts
        .into_iter()
        .map(|s| match s.script_type {
            HoistedScriptType::External => NapiHoistedScript {
                script_type: "external".to_string(),
                src: s.src,
                code: None,
            },
            HoistedScriptType::Inline => NapiHoistedScript {
                script_type: "inline".to_string(),
                code: s.code,
                src: None,
            },
        })
        .collect();

    let hydrated_components = result
        .hydrated_components
        .into_iter()
        .map(|c| NapiHydratedComponent {
            export_name: c.export_name,
            local_name: c.local_name,
            specifier: c.specifier,
            resolved_path: c.resolved_path,
        })
        .collect();

    let client_only_components = result
        .client_only_components
        .into_iter()
        .map(|c| NapiHydratedComponent {
            export_name: c.export_name,
            local_name: c.local_name,
            specifier: c.specifier,
            resolved_path: c.resolved_path,
        })
        .collect();

    let server_components = result
        .server_components
        .into_iter()
        .map(|c| NapiHydratedComponent {
            export_name: c.export_name,
            local_name: c.local_name,
            specifier: c.specifier,
            resolved_path: c.resolved_path,
        })
        .collect();

    AstroCompileResult {
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
/// This transforms an Astro file into JavaScript code compatible with the Astro runtime.
/// The output follows the same format as the official Astro compiler.
///
/// @example
/// ```javascript
/// import { compileAstroSync } from '@astrojs/compiler';
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
#[allow(clippy::needless_pass_by_value, clippy::allow_attributes)]
pub fn compile_astro_sync(
    source_text: String,
    options: Option<AstroCompileOptions>,
) -> AstroCompileResult {
    let options = options.unwrap_or_default();
    compile_astro_impl(&source_text, &options)
}

pub struct AstroCompileTask {
    source_text: String,
    options: AstroCompileOptions,
}

#[napi]
impl Task for AstroCompileTask {
    type JsValue = AstroCompileResult;
    type Output = AstroCompileResult;

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
/// This transforms an Astro file into JavaScript code compatible with the Astro runtime.
/// The output follows the same format as the official Astro compiler.
///
/// Generally `compileAstroSync` is preferable to use as it does not have the overhead
/// of spawning a thread. If you need to parallelize compilation of multiple files,
/// it is recommended to use worker threads.
///
/// @example
/// ```javascript
/// import { compileAstro } from '@astrojs/compiler';
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
    options: Option<AstroCompileOptions>,
) -> AsyncTask<AstroCompileTask> {
    let options = options.unwrap_or_default();
    AsyncTask::new(AstroCompileTask {
        source_text,
        options,
    })
}
