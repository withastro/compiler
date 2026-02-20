//! Unified diagnostic types for the Astro compiler.
//!
//! These types are the source of truth for all compiler diagnostics.
//! Both parse errors (from oxc) and codegen warnings are mapped into this
//! format before reaching the user.

/// Severity level for a diagnostic message.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DiagnosticSeverity {
    Error,
    Warning,
    Information,
    Hint,
}

/// A labeled source span within a diagnostic.
#[derive(Debug, Clone)]
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

impl DiagnosticLabel {
    /// Create a label from byte offsets, computing line/column from source text.
    pub fn new(text: Option<String>, start: u32, end: u32, source_text: &str) -> Self {
        let (line, column) = byte_offset_to_line_column(source_text, start as usize);
        Self {
            text,
            start,
            end,
            line,
            column,
        }
    }
}

/// A single diagnostic message produced by the compiler.
///
/// This is the canonical format — both oxc parse errors and codegen-level
/// warnings/hints are mapped into this shape before reaching the user.
#[derive(Debug, Clone)]
pub struct Diagnostic {
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
    pub fn from_oxc(source_text: &str, diag: &oxc_diagnostics::OxcDiagnostic) -> Self {
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
                    .map(|label| {
                        DiagnosticLabel::new(
                            label.label().map(ToString::to_string),
                            label.offset() as u32,
                            (label.offset() + label.len()) as u32,
                            source_text,
                        )
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
    pub fn from_oxc_list(
        source_text: &str,
        diagnostics: &[oxc_diagnostics::OxcDiagnostic],
    ) -> Vec<Self> {
        diagnostics
            .iter()
            .map(|d| Self::from_oxc(source_text, d))
            .collect()
    }
}

/// Convert a UTF-8 byte offset to a 1-based line and 0-based column.
fn byte_offset_to_line_column(source: &str, offset: usize) -> (u32, u32) {
    let mut line = 1u32;
    let mut col = 0u32;
    for (i, ch) in source.char_indices() {
        if i >= offset {
            break;
        }
        if ch == '\n' {
            line += 1;
            col = 0;
        } else {
            col += 1;
        }
    }
    (line, col)
}
