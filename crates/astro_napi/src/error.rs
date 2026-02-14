use napi_derive::napi;

use astro_codegen::Diagnostic;

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
}

/// A diagnostic message produced by the compiler.
#[napi(object)]
#[derive(Clone)]
pub struct DiagnosticMessage {
    /// Severity level: 1 = error, 2 = warning, 3 = information, 4 = hint.
    pub severity: u32,
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
            severity: d.severity as u32,
            text: d.text,
            hint: d.hint,
            labels: d
                .labels
                .into_iter()
                .map(|l| DiagnosticLabel {
                    text: l.text,
                    start: l.start,
                    end: l.end,
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
