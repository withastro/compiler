//! Unified diagnostic types for the Astro compiler.
//!
//! These types are the source of truth for all compiler diagnostics.
//! Both parse errors (from oxc) and codegen warnings are mapped into this
//! format before reaching the user.

/// Severity level for a diagnostic message.
///
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u32)]
pub enum DiagnosticSeverity {
    Error = 1,
    Warning = 2,
    Information = 3,
    Hint = 4,
}

/// A labeled source span within a diagnostic.
///
/// Mirrors oxc's `LabeledSpan`: a byte range in the source with an optional
/// descriptive message.
#[derive(Debug, Clone)]
pub struct DiagnosticLabel {
    /// Optional label text (e.g. "expected closing tag here").
    pub text: Option<String>,
    /// Byte offset of the span start.
    pub start: u32,
    /// Byte offset of the span end (exclusive).
    pub end: u32,
}

/// A single diagnostic message produced by the compiler.
///
/// This is the canonical format — both oxc parse errors and codegen-level
/// warnings/hints are mapped into this shape before reaching the user.
#[derive(Debug, Clone)]
pub struct Diagnostic {
    /// Severity level (1=error, 2=warning, 3=info, 4=hint).
    pub severity: DiagnosticSeverity,
    /// Human-readable message text.
    pub text: String,
    /// Optional hint/suggestion for fixing the issue.
    pub hint: String,
    /// Labeled source spans.
    pub labels: Vec<DiagnosticLabel>,
}

impl Diagnostic {
    /// Create a diagnostic from an oxc `OxcDiagnostic`.
    ///
    /// Maps oxc severity → our severity, and converts all `LabeledSpan`s.
    pub fn from_oxc(diag: &oxc_diagnostics::OxcDiagnostic) -> Self {
        let severity = match diag.severity {
            oxc_diagnostics::Severity::Error => DiagnosticSeverity::Error,
            oxc_diagnostics::Severity::Warning => DiagnosticSeverity::Warning,
            oxc_diagnostics::Severity::Advice => DiagnosticSeverity::Hint,
        };

        let hint = diag
            .help
            .as_ref()
            .map(ToString::to_string)
            .unwrap_or_default();

        let labels = diag
            .labels
            .as_ref()
            .map(|labels| {
                labels
                    .iter()
                    .map(|label| DiagnosticLabel {
                        text: label.label().map(ToString::to_string),
                        start: label.offset() as u32,
                        end: (label.offset() + label.len()) as u32,
                    })
                    .collect()
            })
            .unwrap_or_default();

        Self {
            severity,
            text: diag.message.to_string(),
            hint,
            labels,
        }
    }

    /// Batch-convert a list of oxc diagnostics.
    pub fn from_oxc_list(diagnostics: &[oxc_diagnostics::OxcDiagnostic]) -> Vec<Self> {
        diagnostics.iter().map(Self::from_oxc).collect()
    }
}
