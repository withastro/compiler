//! Options for Astro codegen.
//!
//! These options mirror the Go Astro compiler's `TransformOptions` from
//! `@astrojs/compiler`. Some fields (such as `compact`, `sourcemap`, CSS scoping)
//! are accepted but stubbed for API compatibility.

/// Scoped style strategy for CSS scoping.
///
/// Determines how Astro scopes CSS selectors to components.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum ScopedStyleStrategy {
    /// Use `:where(.astro-XXXX)` selector (default).
    #[default]
    Where,
    /// Use `.astro-XXXX` class selector.
    Class,
    /// Use `[data-astro-cid-XXXX]` attribute selector.
    Attribute,
}

/// Options for Astro code generation.
///
/// Matches the Go compiler's `TransformOptions` shape from `@astrojs/compiler`.
pub struct TransformOptions {
    /// The filename of the Astro component being compiled.
    /// Used in `$$createComponent` for debugging and scope hash computation.
    pub filename: Option<String>,

    /// A normalized version of the filename used for scope hash generation.
    /// If not provided, falls back to `filename`.
    pub normalized_filename: Option<String>,

    /// The import specifier for Astro runtime functions.
    /// Defaults to `"astro/runtime/server/index.js"`.
    pub internal_url: Option<String>,

    /// Whether to generate a source map.
    ///
    /// Accepts `true`/`false`. In the Go compiler this also accepts
    /// `"inline"`, `"external"`, `"both"` — we currently only support
    /// a boolean (any truthy value enables source map generation).
    ///
    /// **Stub**: source map generation is not yet implemented; this field
    /// is accepted for API compatibility.
    pub sourcemap: bool,

    /// Arguments passed to `$$createAstro` when the Astro global is used.
    /// Defaults to `"https://astro.build"`.
    pub astro_global_args: Option<String>,

    /// Whether to collapse whitespace in the HTML output.
    ///
    /// **Stub**: compact mode is not yet implemented; this field is accepted
    /// for API compatibility.
    pub compact: bool,

    /// Enable scoped slot result handling.
    ///
    /// When `true`, slot callbacks receive the `$$result` render context parameter:
    /// `($$result) => ...` instead of `() => ...`.
    pub result_scoped_slot: bool,

    /// Strategy for CSS scoping.
    ///
    /// **Stub**: CSS scoping is not yet implemented in the Rust compiler;
    /// this is accepted for API compatibility. The value will be used once
    /// CSS scoping support is added.
    pub scoped_style_strategy: ScopedStyleStrategy,

    /// URL for the view transitions animation CSS.
    /// Defaults to `"astro/components/viewtransitions.css"` in the Go compiler.
    ///
    /// **Stub**: accepted for API compatibility.
    pub transitions_animation_url: Option<String>,

    /// Whether to annotate generated code with the source file path.
    ///
    /// **Stub**: accepted for API compatibility.
    pub annotate_source_file: bool,

    /// Enable experimental script ordering behavior.
    ///
    /// **Stub**: accepted for API compatibility.
    pub experimental_script_order: bool,

    /// Whether to strip HTML comments from component slot children.
    ///
    /// When `true` (default), HTML comments inside component children are not
    /// included in slot content. This matches the Go compiler behavior which
    /// explicitly excludes `CommentNode` from slots.
    ///
    /// When `false`, HTML comments are preserved in slot content.
    ///
    /// This only affects comments inside component children (slots), not comments
    /// in regular HTML elements which are always preserved.
    pub strip_slot_comments: bool,

    /// Custom path resolver function.
    ///
    /// When provided (`Some`), it is called for each import specifier to
    /// produce the `resolved_path` on component metadata structs.
    ///
    /// When `None`, falls back to:
    /// - `filepath.join(dir(filename), specifier)` for relative specifiers when filename is set
    /// - The raw specifier unchanged for bare specifiers or when filename is `<stdin>`
    ///
    /// Note: `has_resolve_path()` (used to skip `$$metadata` emission) is true
    /// when either this field is `Some` or `resolve_path_provided` is `true`.
    ///
    /// This mirrors the Go compiler's `ResolvePath func(string) string` option.
    #[expect(clippy::type_complexity)]
    pub resolve_path: Option<Box<dyn Fn(&str) -> String + Send + Sync>>,

    /// Signal that the caller has a `resolvePath` function, even if it's not
    /// passed directly (e.g., it's a JS callback handled post-compilation).
    ///
    /// When `true`, the codegen will skip `$$metadata` emission just like
    /// when `resolve_path` is `Some`, but will still use the filepath.Join
    /// fallback for populating `resolved_path` on metadata structs.
    pub resolve_path_provided: bool,
}

impl Default for TransformOptions {
    fn default() -> Self {
        Self {
            filename: None,
            normalized_filename: None,
            internal_url: None,
            sourcemap: false,
            astro_global_args: None,
            compact: false,
            result_scoped_slot: false,
            scoped_style_strategy: ScopedStyleStrategy::default(),
            transitions_animation_url: None,
            annotate_source_file: false,
            experimental_script_order: false,
            strip_slot_comments: true, // Match Go compiler behavior by default
            resolve_path: None,
            resolve_path_provided: false,
        }
    }
}

impl std::fmt::Debug for TransformOptions {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("TransformOptions")
            .field("filename", &self.filename)
            .field("normalized_filename", &self.normalized_filename)
            .field("internal_url", &self.internal_url)
            .field("sourcemap", &self.sourcemap)
            .field("astro_global_args", &self.astro_global_args)
            .field("compact", &self.compact)
            .field("result_scoped_slot", &self.result_scoped_slot)
            .field("scoped_style_strategy", &self.scoped_style_strategy)
            .field("transitions_animation_url", &self.transitions_animation_url)
            .field("annotate_source_file", &self.annotate_source_file)
            .field("experimental_script_order", &self.experimental_script_order)
            .field("strip_slot_comments", &self.strip_slot_comments)
            .field(
                "resolve_path",
                &self.resolve_path.as_ref().map(|_| "Some(<fn>)"),
            )
            .field("resolve_path_provided", &self.resolve_path_provided)
            .finish()
    }
}

impl TransformOptions {
    /// Create new options with default values.
    pub fn new() -> Self {
        Self::default()
    }

    /// Set the filename.
    #[must_use]
    pub fn with_filename(mut self, filename: impl Into<String>) -> Self {
        self.filename = Some(filename.into());
        self
    }

    /// Set the normalized filename for scope hash generation.
    #[must_use]
    pub fn with_normalized_filename(mut self, filename: impl Into<String>) -> Self {
        self.normalized_filename = Some(filename.into());
        self
    }

    /// Set the internal URL for Astro runtime imports.
    #[must_use]
    pub fn with_internal_url(mut self, url: impl Into<String>) -> Self {
        self.internal_url = Some(url.into());
        self
    }

    /// Enable or disable source map generation (stub).
    #[must_use]
    pub fn with_sourcemap(mut self, enabled: bool) -> Self {
        self.sourcemap = enabled;
        self
    }

    /// Set the Astro global arguments.
    #[must_use]
    pub fn with_astro_global_args(mut self, args: impl Into<String>) -> Self {
        self.astro_global_args = Some(args.into());
        self
    }

    /// Enable or disable compact mode (stub).
    #[must_use]
    pub fn with_compact(mut self, compact: bool) -> Self {
        self.compact = compact;
        self
    }

    /// Enable or disable scoped slot result handling.
    #[must_use]
    pub fn with_result_scoped_slot(mut self, enabled: bool) -> Self {
        self.result_scoped_slot = enabled;
        self
    }

    /// Set the scoped style strategy (stub).
    #[must_use]
    pub fn with_scoped_style_strategy(mut self, strategy: ScopedStyleStrategy) -> Self {
        self.scoped_style_strategy = strategy;
        self
    }

    /// Set the view transitions animation URL (stub).
    #[must_use]
    pub fn with_transitions_animation_url(mut self, url: impl Into<String>) -> Self {
        self.transitions_animation_url = Some(url.into());
        self
    }

    /// Enable or disable source file annotation (stub).
    #[must_use]
    pub fn with_annotate_source_file(mut self, enabled: bool) -> Self {
        self.annotate_source_file = enabled;
        self
    }

    /// Enable or disable experimental script ordering (stub).
    #[must_use]
    pub fn with_experimental_script_order(mut self, enabled: bool) -> Self {
        self.experimental_script_order = enabled;
        self
    }

    /// Set whether to strip HTML comments from component slot children.
    ///
    /// When `true` (default), matches Go compiler behavior by excluding comments from slots.
    /// When `false`, preserves HTML comments in slot content.
    #[must_use]
    pub fn with_strip_slot_comments(mut self, strip: bool) -> Self {
        self.strip_slot_comments = strip;
        self
    }

    /// Set a custom path resolver function.
    ///
    /// When set, the codegen skips `$$metadata`/`$$createMetadata` and resolves
    /// paths at compile time instead of at runtime.
    #[must_use]
    pub fn with_resolve_path(mut self, f: impl Fn(&str) -> String + Send + Sync + 'static) -> Self {
        self.resolve_path = Some(Box::new(f));
        self
    }

    /// Returns true if a `resolve_path` function is provided (directly or signaled).
    ///
    /// When true, the codegen skips `$$metadata`/`$$createMetadata` emission.
    pub fn has_resolve_path(&self) -> bool {
        self.resolve_path.is_some() || self.resolve_path_provided
    }

    /// Resolve an import specifier to a path.
    ///
    /// Uses the custom `resolve_path` function if provided, otherwise falls back to:
    /// - `filepath.join(dir(filename), specifier)` for relative specifiers (starting with `.`)
    /// - The raw specifier unchanged for bare specifiers or when filename is `<stdin>`
    pub fn resolve_specifier(&self, specifier: &str) -> String {
        if let Some(resolve_fn) = &self.resolve_path {
            resolve_fn(specifier)
        } else if let Some(filename) = &self.filename
            && filename != "<stdin>"
            && specifier.starts_with('.')
        {
            // filepath.Join fallback: join the directory of the filename with the specifier
            // then normalize (resolve ../ and ./ segments) to match Go's filepath.Join behavior
            let dir = std::path::Path::new(filename)
                .parent()
                .unwrap_or(std::path::Path::new(""));
            let joined = dir.join(specifier);
            normalize_path(&joined)
        } else {
            specifier.to_string()
        }
    }

    /// Get the internal URL, with default fallback.
    pub fn get_internal_url(&self) -> &str {
        self.internal_url
            .as_deref()
            .unwrap_or("astro/runtime/server/index.js")
    }
}

/// Normalize a path by resolving `.` and `..` segments (without touching the filesystem).
///
/// This mirrors Go's `filepath.Clean` behavior used by `filepath.Join`.
fn normalize_path(path: &std::path::Path) -> String {
    let mut components = Vec::new();
    for component in path.components() {
        match component {
            std::path::Component::CurDir => {} // skip "."
            std::path::Component::ParentDir => {
                // Pop the last component if possible (don't go above root)
                if !components.is_empty()
                    && !matches!(components.last(), Some(std::path::Component::ParentDir))
                {
                    components.pop();
                } else {
                    components.push(component);
                }
            }
            _ => components.push(component),
        }
    }
    let result: std::path::PathBuf = components.iter().collect();
    let s = result.to_string_lossy().to_string();
    // Normalize to forward slashes for consistency
    s.replace('\\', "/")
}

// Keep the old name as a type alias during migration
/// Deprecated: Use [`TransformOptions`] instead.
pub type AstroCodegenOptions = TransformOptions;
