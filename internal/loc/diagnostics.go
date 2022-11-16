package loc

type DiagnosticCode int

const (
	ERROR                             DiagnosticCode = 1000
	ERROR_UNTERMINATED_JS_COMMENT     DiagnosticCode = 1001
	ERROR_FRAGMENT_SHORTHAND_ATTRS    DiagnosticCode = 1002
	ERROR_UNMATCHED_IMPORT            DiagnosticCode = 1003
	ERROR_UNSUPPORTED_SLOT_ATTRIBUTE  DiagnosticCode = 1004
	WARNING                           DiagnosticCode = 2000
	WARNING_UNTERMINATED_HTML_COMMENT DiagnosticCode = 2001
	WARNING_UNCLOSED_HTML_TAG         DiagnosticCode = 2002
	WARNING_DEPRECATED_DIRECTIVE      DiagnosticCode = 2003
	WARNING_IGNORED_DIRECTIVE         DiagnosticCode = 2004
	WARNING_UNSUPPORTED_EXPRESSION    DiagnosticCode = 2005
	WARNING_SET_WITH_CHILDREN         DiagnosticCode = 2006
	WARNING_CANNOT_DEFINE_VARS        DiagnosticCode = 2007
	WARNING_INVALID_SPREAD            DiagnosticCode = 2008
	INFO                              DiagnosticCode = 3000
	HINT                              DiagnosticCode = 4000
)
