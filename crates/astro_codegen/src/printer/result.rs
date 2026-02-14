//! Public output types for Astro code generation.
//!
//! These types represent the result of transforming an Astro component.

use crate::diagnostic::Diagnostic;

/// The type of a hoisted script.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum HoistedScriptType {
    /// An inline script with code content.
    Inline,
    /// An external script with a `src` URL.
    External,
}

/// A hoisted script extracted from the Astro template.
#[derive(Debug, Clone)]
pub struct TransformResultHoistedScript {
    /// The script type: `"inline"` or `"external"`.
    pub script_type: HoistedScriptType,
    /// The inline script code (when `script_type` is `Inline`).
    pub code: Option<String>,
    /// The external script src URL (when `script_type` is `External`).
    pub src: Option<String>,
    // TODO: add a `map` field for inline scripts (sourcemap).
}

/// A hydrated component reference found in the template.
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
#[derive(Debug)]
pub struct TransformResult {
    /// The generated JavaScript code.
    pub code: String,
    /// Source map JSON string.
    ///
    /// Contains a JSON-encoded source map when `TransformOptions::sourcemap` is `true`.
    /// Empty string when sourcemap generation is disabled.
    pub map: String,
    /// CSS scope hash for the component (e.g., `"astro-XXXXXX"`).
    pub scope: String,
    /// Extracted CSS strings from `<style>` tags (stub: empty vec until CSS support).
    pub style_error: Vec<String>,
    /// Diagnostic messages from compilation.
    pub diagnostics: Vec<Diagnostic>,
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
