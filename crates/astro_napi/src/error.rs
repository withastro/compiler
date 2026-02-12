use std::sync::Arc;

use napi_derive::napi;

use oxc_diagnostics::{LabeledSpan, NamedSource, OxcDiagnostic};

#[napi(object, use_nullable = true)]
#[derive(Clone)]
pub struct OxcError {
    pub severity: Severity,
    pub message: String,
    pub labels: Vec<ErrorLabel>,
    pub help_message: Option<String>,
    pub codeframe: Option<String>,
}

impl OxcError {
    pub fn new(message: String) -> Self {
        Self {
            severity: Severity::Error,
            message,
            labels: vec![],
            help_message: None,
            codeframe: None,
        }
    }

    pub fn from_diagnostics(
        filename: &str,
        source_text: &str,
        diagnostics: Vec<OxcDiagnostic>,
    ) -> Vec<Self> {
        if diagnostics.is_empty() {
            return vec![];
        }
        let source = Arc::new(NamedSource::new(filename, source_text.to_string()));
        diagnostics
            .into_iter()
            .map(|e| Self::from_diagnostic(source_text, &source, e))
            .collect()
    }

    pub fn from_diagnostic(
        source_text: &str,
        named_source: &Arc<NamedSource<String>>,
        diagnostic: OxcDiagnostic,
    ) -> Self {
        let severity = Severity::from(diagnostic.severity);
        let message = diagnostic.message.to_string();
        let help_message = diagnostic.help.as_ref().map(ToString::to_string);
        let labels = diagnostic
            .labels
            .as_ref()
            .map(|labels| {
                labels
                    .iter()
                    .map(|label| ErrorLabel::new(label, source_text))
                    .collect::<Vec<_>>()
            })
            .unwrap_or_default();
        let codeframe = diagnostic.with_source_code(Arc::clone(named_source));
        Self {
            severity,
            message,
            labels,
            help_message,
            codeframe: Some(format!("{codeframe:?}")),
        }
    }
}

#[napi(object, use_nullable = true)]
#[derive(Clone)]
pub struct ErrorLabel {
    pub message: Option<String>,
    pub start: u32,
    pub end: u32,
    /// 1-based line number in the source.
    pub line: u32,
    /// 0-based column number in the source.
    pub column: u32,
}

impl ErrorLabel {
    #[expect(clippy::cast_possible_truncation)]
    pub fn new(label: &LabeledSpan, source_text: &str) -> Self {
        let start = label.offset();
        let end = start + label.len();
        let (line, column) = byte_offset_to_line_column(source_text, start);
        Self {
            message: label.label().map(ToString::to_string),
            start: start as u32,
            end: end as u32,
            line,
            column,
        }
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

#[napi(string_enum)]
#[derive(Clone)]
pub enum Severity {
    Error,
    Warning,
    Advice,
}

impl From<oxc_diagnostics::Severity> for Severity {
    fn from(value: oxc_diagnostics::Severity) -> Self {
        match value {
            oxc_diagnostics::Severity::Error => Self::Error,
            oxc_diagnostics::Severity::Warning => Self::Warning,
            oxc_diagnostics::Severity::Advice => Self::Advice,
        }
    }
}
