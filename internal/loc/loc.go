package loc

type Loc struct {
	// This is the 0-based index of this location from the start of the file, in bytes
	Start int
}

type Range struct {
	Loc Loc
	Len int
}

func (r Range) End() int {
	return r.Loc.Start + r.Len
}

// span is a range of bytes in a Tokenizer's buffer. The start is inclusive,
// the end is exclusive.
type Span struct {
	Start, End int
}

// A NodeType is the type of a Node.
type DiagnosticSeverity int

const (
	ErrorType DiagnosticSeverity = 1
	WarningType
	InformationType
	HintType
)

type DiagnosticMessage struct {
	Severity int                 `js:"severity"`
	Code     int                 `js:"code"`
	Location *DiagnosticLocation `js:"location"`
	Hint     string              `js:"hint"`
	Text     string              `js:"text"`
}

type DiagnosticLocation struct {
	File   string `js:"file"`
	Line   int    `js:"line"`
	Column int    `js:"column"`
	Length int    `js:"length"`
}

type ErrorWithRange struct {
	Code  DiagnosticCode
	Text  string
	Hint  string
	Range Range
}

func (e *ErrorWithRange) Error() string {
	return e.Text
}

func (e *ErrorWithRange) ToMessage(location *DiagnosticLocation) DiagnosticMessage {
	return DiagnosticMessage{
		Code:     int(e.Code),
		Text:     e.Error(),
		Hint:     e.Hint,
		Location: location,
	}
}
