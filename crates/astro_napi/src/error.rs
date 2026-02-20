use napi_derive::napi;

use astro_codegen::Diagnostic;

/// Severity level for a diagnostic message.
#[napi(string_enum)]
#[derive(Clone)]
pub enum DiagnosticSeverity {
    #[napi(value = "error")]
    Error,
    #[napi(value = "warning")]
    Warning,
    #[napi(value = "information")]
    Information,
    #[napi(value = "hint")]
    Hint,
}

impl From<astro_codegen::DiagnosticSeverity> for DiagnosticSeverity {
    fn from(s: astro_codegen::DiagnosticSeverity) -> Self {
        match s {
            astro_codegen::DiagnosticSeverity::Error => Self::Error,
            astro_codegen::DiagnosticSeverity::Warning => Self::Warning,
            astro_codegen::DiagnosticSeverity::Information => Self::Information,
            astro_codegen::DiagnosticSeverity::Hint => Self::Hint,
        }
    }
}

/// A labeled source span within a diagnostic.
#[napi(object, use_nullable = true)]
#[derive(Clone)]
pub struct DiagnosticLabel {
    /// Optional label text (e.g. "expected closing tag here").
    pub text: Option<String>,
    /// Byte offset of the span start.
    pub start: u32,
    /// Byte offset of the span end (exclusive).
    pub end: u32,
    /// 1-based line number.
    pub line: u32,
    /// 0-based column number.
    pub column: u32,
}

/// A diagnostic message produced by the compiler.
#[napi(object)]
#[derive(Clone)]
pub struct DiagnosticMessage {
    #[napi(ts_type = "'error' | 'warning' | 'information' | 'hint'")]
    pub severity: DiagnosticSeverity,
    /// Human-readable message text.
    pub text: String,
    /// Optional hint/suggestion for fixing the issue.
    pub hint: String,
    /// Labeled source spans.
    pub labels: Vec<DiagnosticLabel>,
}

impl From<Diagnostic> for DiagnosticMessage {
    fn from(d: Diagnostic) -> Self {
        Self {
            severity: DiagnosticSeverity::from(d.severity),
            text: d.text,
            hint: d.hint,
            labels: d
                .labels
                .into_iter()
                .map(|l| DiagnosticLabel {
                    text: l.text,
                    start: l.start,
                    end: l.end,
                    line: l.line,
                    column: l.column,
                })
                .collect(),
        }
    }
}

impl DiagnosticMessage {
    /// Convert a list of codegen diagnostics to NAPI diagnostic messages.
    pub fn from_codegen_list(diagnostics: Vec<Diagnostic>) -> Vec<Self> {
        diagnostics.into_iter().map(Self::from).collect()
    }
}
