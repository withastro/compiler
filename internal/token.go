// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package astro

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/withastro/compiler/internal/handler"
	"github.com/withastro/compiler/internal/loc"
	"golang.org/x/net/html/atom"
)

// A TokenType is the type of a Token.
type TokenType uint32

const (
	// ErrorToken means that an error occurred during tokenization.
	ErrorToken TokenType = iota
	// TextToken means a text node.
	TextToken
	// A StartTagToken looks like <a>.
	StartTagToken
	// An EndTagToken looks like </a>.
	EndTagToken
	// A SelfClosingTagToken tag looks like <br/>.
	SelfClosingTagToken
	// A CommentToken looks like <!--x-->.
	CommentToken
	// A DoctypeToken looks like <!DOCTYPE x>
	DoctypeToken

	// ASTRO EXTENSIONS
	// A FenceToken is the opening or closing --- of Frontmatter
	FrontmatterFenceToken

	// A StartExpressionToken looks like { and can contain
	StartExpressionToken
	// An EndExpressionToken looks like }
	EndExpressionToken
)

// FrontmatterState tracks the open/closed state of Frontmatter.
type FrontmatterState uint32

const (
	FrontmatterInitial FrontmatterState = iota
	FrontmatterOpen
	FrontmatterClosed
)

// AttributeType is the type of an Attribute
type AttributeType uint32

func (t AttributeType) String() string {
	switch t {
	case QuotedAttribute:
		return "quoted"
	case EmptyAttribute:
		return "empty"
	case ExpressionAttribute:
		return "expression"
	case SpreadAttribute:
		return "spread"
	case ShorthandAttribute:
		return "shorthand"
	case TemplateLiteralAttribute:
		return "template-literal"
	}
	return "Invalid(" + strconv.Itoa(int(t)) + ")"
}

const (
	QuotedAttribute AttributeType = iota
	EmptyAttribute
	ExpressionAttribute
	SpreadAttribute
	ShorthandAttribute
	TemplateLiteralAttribute
)

// ErrBufferExceeded means that the buffering limit was exceeded.
var ErrBufferExceeded = errors.New("max buffer exceeded")

// String returns a string representation of the TokenType.
func (t TokenType) String() string {
	switch t {
	case ErrorToken:
		return "Error"
	case TextToken:
		return "Text"
	case StartTagToken:
		return "StartTag"
	case EndTagToken:
		return "EndTag"
	case SelfClosingTagToken:
		return "SelfClosingTag"
	case CommentToken:
		return "Comment"
	case DoctypeToken:
		return "Doctype"
	case FrontmatterFenceToken:
		return "FrontmatterFence"
	case StartExpressionToken:
		return "StartExpression"
	case EndExpressionToken:
		return "EndExpression"
	}
	return "Invalid(" + strconv.Itoa(int(t)) + ")"
}

func (fm FrontmatterState) String() string {
	switch fm {
	case FrontmatterInitial:
		return "Initial"
	case FrontmatterOpen:
		return "Open"
	case FrontmatterClosed:
		return "Closed"
	}
	return "Invalid(" + strconv.Itoa(int(fm)) + ")"
}

// An Attribute is an attribute namespace-key-value triple. Namespace is
// non-empty for foreign attributes like xlink, Key is alphabetic (and hence
// does not contain escapable characters like '&', '<' or '>'), and Val is
// unescaped (it looks like "a<b" rather than "a&lt;b").
//
// Namespace is only used by the parser, not the tokenizer.
type Attribute struct {
	Namespace string
	Key       string
	KeyLoc    loc.Loc
	Val       string
	ValLoc    loc.Loc
	Tokenizer *Tokenizer
	Type      AttributeType
}

type Expression struct {
	Data     string
	Children []Token
	Loc      loc.Loc
}

// A Token consists of a TokenType and some Data (tag name for start and end
// tags, content for text, comments and doctypes). A tag Token may also contain
// a slice of Attributes. Data is unescaped for all Tokens (it looks like "a<b"
// rather than "a&lt;b"). For tag Tokens, DataAtom is the atom for Data, or
// zero if Data is not a known tag name.
type Token struct {
	Type     TokenType
	DataAtom atom.Atom
	Data     string
	Attr     []Attribute
	Loc      loc.Loc
}

// tagString returns a string representation of a tag Token's Data and Attr.
func (t Token) tagString() string {
	if len(t.Attr) == 0 {
		return t.Data
	}
	buf := bytes.NewBufferString(t.Data)
	for _, a := range t.Attr {
		buf.WriteByte(' ')

		switch a.Type {
		case QuotedAttribute:
			buf.WriteString(a.Key)
			buf.WriteString(`="`)
			escape(buf, a.Val)
			buf.WriteByte('"')
		case EmptyAttribute:
			buf.WriteString(a.Key)
		case ExpressionAttribute:
			buf.WriteString(a.Key)
			buf.WriteString(`={`)
			buf.WriteString(a.Val)
			buf.WriteByte('}')
		case TemplateLiteralAttribute:
			buf.WriteString(a.Key)
			buf.WriteByte('=')
			buf.WriteByte('{')
			buf.WriteByte('`')
			escape(buf, a.Val)
			buf.WriteByte('`')
			buf.WriteByte('}')
		case ShorthandAttribute:
			buf.WriteByte('{')
			buf.WriteString(a.Key)
			buf.WriteByte('}')
		case SpreadAttribute:
			buf.WriteString("{...")
			buf.WriteString(a.Key)
			buf.WriteByte('}')
		default:
			buf.WriteString(a.Key)
		}
	}
	return buf.String()
}

// String returns a string representation of the Token.
func (t Token) String() string {
	switch t.Type {
	case ErrorToken:
		return ""
	case TextToken:
		return EscapeString(t.Data)
	case StartTagToken:
		return "<" + t.tagString() + ">"
	case EndTagToken:
		return "</" + t.tagString() + ">"
	case SelfClosingTagToken:
		return "<" + t.tagString() + "/>"
	case CommentToken:
		return "<!--" + t.Data + "-->"
	case DoctypeToken:
		return "<!DOCTYPE " + t.Data + ">"
	case FrontmatterFenceToken:
		return "---"
	case StartExpressionToken:
		return "{"
	case EndExpressionToken:
		return "}"
	}
	return "Invalid(" + strconv.Itoa(int(t.Type)) + ")"
}

// A Tokenizer returns a stream of HTML Tokens.
type Tokenizer struct {
	// r is the source of the HTML text.
	r io.Reader
	// tt is the TokenType of the current token.
	tt        TokenType
	prevToken Token
	fm        FrontmatterState
	// err is the first error encountered during tokenization. It is possible
	// for tt != Error && err != nil to hold: this means that Next returned a
	// valid token but the subsequent Next call will return an error token.
	// For example, if the HTML text input was just "plain", then the first
	// Next call would set z.err to io.EOF but return a TextToken, and all
	// subsequent Next calls would return an ErrorToken.
	// err is never reset. Once it becomes non-nil, it stays non-nil.
	err error
	// buf[raw.Start:raw.End] holds the raw bytes of the current token.
	// buf[raw.End:] is buffered input that will yield future tokens.
	raw loc.Span
	buf []byte
	// buf[data.Start:data.End] holds the raw bytes of the current token's data:
	// a text token's text, a tag token's tag name, etc.
	data loc.Span
	// pendingAttr is the attribute key and value currently being tokenized.
	// When complete, pendingAttr is pushed onto attr. nAttrReturned is
	// incremented on each call to TagAttr.
	pendingAttr              [2]loc.Span
	pendingAttrType          AttributeType
	attr                     [][2]loc.Span
	attrTypes                []AttributeType
	attrExpressionStack      int
	attrTemplateLiteralStack []int

	nAttrReturned int
	dashCount     int
	// expressionStack is an array of counters tracking opening and closing
	// braces in nested expressions
	expressionStack            []int
	expressionElementStack     [][]string
	openBraceIsExpressionStart bool
	// rawTag is the "script" in "</script>" that closes the next token. If
	// non-empty, the subsequent call to Next will return a raw or RCDATA text
	// token: one that treats "<p>" as text instead of an element.
	// rawTag's contents are lower-cased.
	rawTag string
	// noExpressionTag is the "math" in "<math>". If non-empty, any instances
	// of "{" will be treated as raw text rather than an StartExpressionToken.
	// noExpressionTag's contents are lower-cased.
	noExpressionTag string
	// stringStartChar is the character that opened the last string: ', ", or `
	// stringStartChar byte
	// stringIsOpen will be true while in the context of a string
	// stringIsOpen bool
	// textIsRaw is whether the current text token's data is not escaped.
	textIsRaw bool
	// convertNUL is whether NUL bytes in the current token's data should
	// be converted into \ufffd replacement characters.
	convertNUL bool
	// allowCDATA is whether CDATA sections are allowed in the current context.
	allowCDATA bool

	handler *handler.Handler
}

// AllowCDATA sets whether or not the tokenizer recognizes <![CDATA[foo]]> as
// the text "foo". The default value is false, which means to recognize it as
// a bogus comment "<!-- [CDATA[foo]] -->" instead.
//
// Strictly speaking, an HTML5 compliant tokenizer should allow CDATA if and
// only if tokenizing foreign content, such as MathML and SVG. However,
// tracking foreign-contentness is difficult to do purely in the tokenizer,
// as opposed to the parser, due to HTML integration points: an <svg> element
// can contain a <foreignObject> that is foreign-to-SVG but not foreign-to-
// HTML. For strict compliance with the HTML5 tokenization algorithm, it is the
// responsibility of the user of a tokenizer to call AllowCDATA as appropriate.
// In practice, if using the tokenizer without caring whether MathML or SVG
// CDATA is text or comments, such as tokenizing HTML to find all the anchor
// text, it is acceptable to ignore this responsibility.
func (z *Tokenizer) AllowCDATA(allowCDATA bool) {
	z.allowCDATA = allowCDATA
}

// NextIsNotRawText instructs the tokenizer that the next token should not be
// considered as 'raw text'. Some elements, such as script and title elements,
// normally require the next token after the opening tag to be 'raw text' that
// has no child elements. For example, tokenizing "<title>a<b>c</b>d</title>"
// yields a start tag token for "<title>", a text token for "a<b>c</b>d", and
// an end tag token for "</title>". There are no distinct start tag or end tag
// tokens for the "<b>" and "</b>".
//
// The only exception is <style>, which should be treated as raw text no
// matter what (handled in the conditional).
//
// This tokenizer implementation will generally look for raw text at the right
// times. Strictly speaking, an HTML5 compliant tokenizer should not look for
// raw text if in foreign content: <title> generally needs raw text, but a
// <title> inside an <svg> does not. Another example is that a <textarea>
// generally needs raw text, but a <textarea> is not allowed as an immediate
// child of a <select>; in normal parsing, a <textarea> implies </select>, but
// one cannot close the implicit element when parsing a <select>'s InnerHTML.
// Similarly to AllowCDATA, tracking the correct moment to override raw-text-
// ness is difficult to do purely in the tokenizer, as opposed to the parser.
// For strict compliance with the HTML5 tokenization algorithm, it is the
// responsibility of the user of a tokenizer to call NextIsNotRawText as
// appropriate. In practice, like AllowCDATA, it is acceptable to ignore this
// responsibility for basic usage.
//
// Note that this 'raw text' concept is different from the one offered by the
// Tokenizer.Raw method.
func (z *Tokenizer) NextIsNotRawText() {
	if z.rawTag != "style" {
		z.rawTag = ""
	}
}

// Err returns the error associated with the most recent ErrorToken token.
// This is typically io.EOF, meaning the end of tokenization.
func (z *Tokenizer) Err() error {
	if z.tt != ErrorToken {
		return nil
	}
	return z.err
}

// readByte returns the next byte from the input buffer.
// z.buf[z.raw.Start:z.raw.End] remains a contiguous byte
// slice that holds all the bytes read so far for the current token.
// Pre-condition: z.err == nil.
func (z *Tokenizer) readByte() byte {
	if z.raw.End >= len(z.buf) {
		z.err = io.EOF // note: io.EOF is the only “safe” error that is a signal for the compiler to exit cleanly
		return 0
	}
	x := z.buf[z.raw.End]
	z.raw.End++
	return x
}

// Buffered returns a slice containing data buffered but not yet tokenized.
func (z *Tokenizer) Buffered() []byte {
	return z.buf[z.raw.End:]
}

// skipWhiteSpace skips past any white space.
func (z *Tokenizer) skipWhiteSpace() {
	if z.err != nil {
		return
	}
	for {
		c := z.readByte()
		if z.err != nil {
			if z.err == io.EOF {
				return
			}
			z.handler.AppendWarning(&loc.ErrorWithRange{
				Code: loc.WARNING_UNEXPECTED_CHARACTER,
				Text: fmt.Sprintf("Unexpected character in skipWhiteSpace: \"%v\"\n", string(c)),
				Range: loc.Range{
					Loc: loc.Loc{Start: z.raw.End - 1},
					Len: 1,
				},
			})
			return
		}
		if !unicode.IsSpace(rune(c)) {
			z.raw.End--
			return
		}
	}
}

// readRawOrRCDATA reads until the next "</foo>", where "foo" is z.rawTag and
// is typically something like "script" or "textarea".
func (z *Tokenizer) readRawOrRCDATA() {
	// If <script /> or any raw tag, don't try to read any data
	if z.Token().Type == SelfClosingTagToken {
		z.data.End = z.raw.End
		z.rawTag = ""
		z.noExpressionTag = ""
		return
	}
	if z.rawTag == "script" {
		z.readScript()
		z.textIsRaw = true
		z.rawTag = ""
		z.noExpressionTag = ""
		return
	}
loop:
	for {
		c := z.readByte()
		if z.err != nil {
			if z.err == io.EOF {
				return
			}
			z.handler.AppendWarning(&loc.ErrorWithRange{
				Code: loc.WARNING_UNEXPECTED_CHARACTER,
				Text: fmt.Sprintf("Unexpected character in loop: \"%v\"\n", string(c)),
				Range: loc.Range{
					Loc: loc.Loc{Start: z.raw.End - 1},
					Len: 1,
				},
			})
			break loop
		}
		if c != '<' {
			continue loop
		}
		c = z.readByte()
		if z.err != nil {
			break loop
		}
		if c != '/' {
			z.raw.End--
			continue loop
		}
		if z.readRawEndTag() || z.err != nil {
			break loop
		}
	}
	z.data.End = z.raw.End
	// A textarea's or title's RCDATA can contain escaped entities.
	z.textIsRaw = z.rawTag != "textarea" && z.rawTag != "title"
	z.rawTag = ""
}

// readRawEndTag attempts to read a tag like "</foo>", where "foo" is z.rawTag.
// If it succeeds, it backs up the input position to reconsume the tag and
// returns true. Otherwise it returns false. The opening "</" has already been
// consumed.
func (z *Tokenizer) readRawEndTag() bool {
	for i := 0; i < len(z.rawTag); i++ {
		c := z.readByte()
		if z.err != nil {
			return false
		}
		if c != z.rawTag[i] && c != z.rawTag[i]-('a'-'A') {
			z.raw.End--
			return false
		}
	}
	c := z.readByte()
	if z.err != nil {
		if z.err == io.EOF {
			return false
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in readRawEndTag: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return false
	}
	switch c {
	case ' ', '\n', '\r', '\t', '\f', '/', '>':
		// The 3 is 2 for the leading "</" plus 1 for the trailing character c.
		z.raw.End -= 3 + len(z.rawTag)
		return true
	}
	z.raw.End--
	return false
}

// readScript reads until the next </script> tag, following the byzantine
// rules for escaping/hiding the closing tag.
func (z *Tokenizer) readScript() {
	defer func() {
		z.data.End = z.raw.End
	}()
	var c byte

scriptData:
	c = z.readByte()
	if z.err != nil {
		if z.err == io.EOF {
			return
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in scriptData: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return
	}
	if c == '<' {
		goto scriptDataLessThanSign
	}
	goto scriptData

scriptDataLessThanSign:
	c = z.readByte()
	if z.err != nil {
		if z.err == io.EOF {
			return
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in scriptDataLessThanSign: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return
	}
	switch c {
	case '/':
		goto scriptDataEndTagOpen
	case '!':
		goto scriptDataEscapeStart
	}
	z.raw.End--
	goto scriptData

scriptDataEndTagOpen:
	if z.err != nil {
		if z.err == io.EOF {
			return
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in scriptDataEndTagOpen: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return
	}
	if z.readRawEndTag() {
		return
	}
	goto scriptData

scriptDataEscapeStart:
	c = z.readByte()
	if z.err != nil {
		if z.err == io.EOF {
			return
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in scriptDataEscapeStart: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return
	}
	if c == '-' {
		goto scriptDataEscapeStartDash
	}
	z.raw.End--
	goto scriptData

scriptDataEscapeStartDash:
	c = z.readByte()
	if z.err != nil {
		if z.err == io.EOF {
			return
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in scriptDataEscapeStartDash: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return
	}
	if c == '-' {
		goto scriptDataEscapedDashDash
	}
	z.raw.End--
	goto scriptData

scriptDataEscaped:
	c = z.readByte()
	if z.err != nil {
		if z.err == io.EOF {
			return
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in scriptDataEscaped: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return
	}
	switch c {
	case '-':
		goto scriptDataEscapedDash
	case '<':
		goto scriptDataEscapedLessThanSign
	}
	goto scriptDataEscaped

scriptDataEscapedDash:
	c = z.readByte()
	if z.err != nil {
		if z.err == io.EOF {
			return
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in scriptDataEscapedDash: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return
	}
	switch c {
	case '-':
		goto scriptDataEscapedDashDash
	case '<':
		goto scriptDataEscapedLessThanSign
	}
	goto scriptDataEscaped

scriptDataEscapedDashDash:
	c = z.readByte()
	if z.err != nil {
		if z.err == io.EOF {
			return
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in scriptDataEscapedDashDash: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return
	}
	switch c {
	case '-':
		goto scriptDataEscapedDashDash
	case '<':
		goto scriptDataEscapedLessThanSign
	case '>':
		goto scriptData
	}
	goto scriptDataEscaped

scriptDataEscapedLessThanSign:
	c = z.readByte()
	if z.err != nil {
		if z.err == io.EOF {
			return
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in scriptDataEscapedLessThanSign: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return
	}
	if c == '/' {
		goto scriptDataEscapedEndTagOpen
	}
	if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' {
		goto scriptDataDoubleEscapeStart
	}
	z.raw.End--
	goto scriptData

scriptDataEscapedEndTagOpen:
	if z.err != nil {
		if z.err == io.EOF {
			return
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in scriptDataEscapedEndTagOpen: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return
	}
	if z.readRawEndTag() || z.err != nil {
		return
	}
	goto scriptDataEscaped

scriptDataDoubleEscapeStart:
	z.raw.End--
	for i := 0; i < len("script"); i++ {
		c = z.readByte()
		if z.err != nil {
			if z.err == io.EOF {
				return
			}
			z.handler.AppendWarning(&loc.ErrorWithRange{
				Code: loc.WARNING_UNEXPECTED_CHARACTER,
				Text: fmt.Sprintf("Unexpected character in scriptDataDoubleEscapeStart: %v\n", string(c)),
				Range: loc.Range{
					Loc: loc.Loc{Start: z.raw.End - 1},
					Len: 1,
				},
			})
			return
		}
		if c != "script"[i] && c != "SCRIPT"[i] {
			z.raw.End--
			goto scriptDataEscaped
		}
	}
	c = z.readByte()
	if z.err != nil {
		return
	}
	switch c {
	case ' ', '\n', '\r', '\t', '\f', '/', '>':
		goto scriptDataDoubleEscaped
	}
	z.raw.End--
	goto scriptDataEscaped

scriptDataDoubleEscaped:
	c = z.readByte()
	if z.err != nil {
		if z.err == io.EOF {
			return
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in scriptDataDoubleEscaped: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return
	}
	switch c {
	case '-':
		goto scriptDataDoubleEscapedDash
	case '<':
		goto scriptDataDoubleEscapedLessThanSign
	}
	goto scriptDataDoubleEscaped

scriptDataDoubleEscapedDash:
	c = z.readByte()
	if z.err != nil {
		if z.err == io.EOF {
			return
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in scriptDataDoubleEscapedDash: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return
	}
	switch c {
	case '-':
		goto scriptDataDoubleEscapedDashDash
	case '<':
		goto scriptDataDoubleEscapedLessThanSign
	}
	goto scriptDataDoubleEscaped

scriptDataDoubleEscapedDashDash:
	c = z.readByte()
	if z.err != nil {
		if z.err == io.EOF {
			return
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in scriptDataDoubleEscapedDashDash: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return
	}
	switch c {
	case '-':
		goto scriptDataDoubleEscapedDashDash
	case '<':
		goto scriptDataDoubleEscapedLessThanSign
	case '>':
		goto scriptData
	}
	goto scriptDataDoubleEscaped

scriptDataDoubleEscapedLessThanSign:
	c = z.readByte()
	if z.err != nil {
		if z.err == io.EOF {
			return
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in scriptDataDoubleEscapedLessThanSign: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return
	}
	if c == '/' {
		goto scriptDataDoubleEscapeEnd
	}
	z.raw.End--
	goto scriptDataDoubleEscaped

scriptDataDoubleEscapeEnd:
	if z.readRawEndTag() {
		z.raw.End += len("</script>")
		goto scriptDataEscaped
	}
	if z.err != nil {
		if z.err == io.EOF {
			return
		}
		z.handler.AppendWarning(&loc.ErrorWithRange{
			Code: loc.WARNING_UNEXPECTED_CHARACTER,
			Text: fmt.Sprintf("Unexpected character in scriptDataDoubleEscapeEnd: %v\n", string(c)),
			Range: loc.Range{
				Loc: loc.Loc{Start: z.raw.End - 1},
				Len: 1,
			},
		})
		return
	}
	goto scriptDataDoubleEscaped
}

// readHTMLComment reads the next comment token starting with "<!--". The opening
// "<!--" has already been consumed.
func (z *Tokenizer) readHTMLComment() {
	start := z.raw.End
	z.data.Start = start
	defer func() {
		if z.data.End < z.data.Start {
			// It's a comment with no data, like <!-->.
			z.data.End = z.data.Start
		}
	}()
	for dashCount := 2; ; {
		c := z.readByte()
		if z.err != nil {
			if z.err == io.EOF {
				z.handler.AppendWarning(&loc.ErrorWithRange{
					Code: loc.WARNING_UNTERMINATED_HTML_COMMENT,
					Text: `Unterminated comment`,
					Range: loc.Range{
						Loc: loc.Loc{Start: start},
						Len: 4,
					},
				})
			}
			// Ignore up to two dashes at EOF.
			if dashCount > 2 {
				dashCount = 2
			}
			z.data.End = z.raw.End - dashCount
			return
		}
		switch c {
		case '-':
			dashCount++
			continue
		case '>':
			if dashCount >= 2 {
				z.data.End = z.raw.End - len("-->")
				return
			}
		case '!':
			if dashCount >= 2 {
				c = z.readByte()
				if z.err != nil {
					z.data.End = z.raw.End
					return
				}
				if c == '>' {
					z.data.End = z.raw.End - len("--!>")
					return
				}
			}
		}
		dashCount = 0
	}
}

// readUntilCloseAngle reads until the next ">".
func (z *Tokenizer) readUntilCloseAngle() {
	z.data.Start = z.raw.End
	for {
		c := z.readByte()
		if z.err != nil {
			z.data.End = z.raw.End
			return
		}
		if c == '>' {
			z.data.End = z.raw.End - len(">")
			return
		}
	}
}

// readString reads until a JavaScript string is closed.
func (z *Tokenizer) readString(c byte) {
	switch c {
	// single quote (ends on newline)
	case '\'':
		z.readUntilChar([]byte{'\'', '\r', '\n'})
	// double quote (ends on newline)
	case '"':
		z.readUntilChar([]byte{'"', '\r', '\n'})
	// template literal
	case '`':
		// Note that we DO NOT have to handle `${}` here because our expression
		// behavior already handles `{}` and `z.readTagAttrExpression()` handles
		// template literals separately.
		z.readUntilChar([]byte{'`'})
	}
}

// generic utility to look ahead until the first char is encountered from given splice
func (z *Tokenizer) readUntilChar(chars []byte) {
find_next:
	for {
		c := z.readByte()
		// fail on error
		if z.err != nil {
			z.data.End = z.raw.End - 1
			return
		}
		// handle escape char \
		if c == '\\' {
			z.raw.End++
			c = z.buf[z.data.Start : z.data.Start+1][0]
			// if this is a match but it’s escaped, skip and move to the next char
			for _, v := range chars {
				if c == v {
					z.raw.End++
					continue find_next
				}
			}
		}
		// match found!
		for _, v := range chars {
			if c == v {
				z.data.End = z.raw.End
				return
			}
		}
	}
}

// read RegExp expressions and comments (starting from '/' byte)
func (z *Tokenizer) readCommentOrRegExp(boundaryChars []byte) {
	c := z.readByte() // find next character after '/' to know how to handle it
	switch c {
	// single-line comment (ends on newline)
	case '/':
		z.readUntilChar([]byte{'\r', '\n'})
	// multi-line comment
	case '*':
		start := z.data.Start
		prev := c
		for {
			c = z.readByte()
			if z.err != nil {
				if z.err == io.EOF {
					z.handler.AppendError(&loc.ErrorWithRange{
						Code: loc.ERROR_UNTERMINATED_JS_COMMENT,
						Text: `Unterminated comment`,
						Range: loc.Range{
							Loc: loc.Loc{Start: start},
							Len: 2,
						},
					})
				}
				return
			}
			// look for "*/"
			if prev == '*' && c == '/' {
				z.data.End = z.raw.End
				return
			}
			prev = c
		}
	// RegExp
	default:
		z.raw.End--
		z.readUntilChar(append([]byte{'/', '\r', '\n'}, boundaryChars...))
	}
}

// readMarkupDeclaration reads the next token starting with "<!". It might be
// a "<!--comment-->", a "<!DOCTYPE foo>", a "<![CDATA[section]]>" or
// "<!a bogus comment". The opening "<!" has already been consumed.
func (z *Tokenizer) readMarkupDeclaration() TokenType {
	z.data.Start = z.raw.End
	var c [2]byte
	for i := 0; i < 2; i++ {
		c[i] = z.readByte()
		if z.err != nil {
			z.data.End = z.raw.End
			return CommentToken
		}
	}
	if c[0] == '-' && c[1] == '-' {
		z.readHTMLComment()
		return CommentToken
	}
	z.raw.End -= 2
	if z.readDoctype() {
		return DoctypeToken
	}
	if z.allowCDATA && z.readCDATA() {
		z.convertNUL = true
		return TextToken
	}
	// It's a bogus comment.
	z.readUntilCloseAngle()
	return CommentToken
}

// readDoctype attempts to read a doctype declaration and returns true if
// successful. The opening "<!" has already been consumed.
func (z *Tokenizer) readDoctype() bool {
	const s = "DOCTYPE"
	for i := 0; i < len(s); i++ {
		c := z.readByte()
		if z.err != nil {
			z.data.End = z.raw.End
			return false
		}
		if c != s[i] && c != s[i]+('a'-'A') {
			// Back up to read the fragment of "DOCTYPE" again.
			z.raw.End = z.data.Start
			return false
		}
	}
	if z.skipWhiteSpace(); z.err != nil {
		z.data.Start = z.raw.End
		z.data.End = z.raw.End
		return true
	}
	z.readUntilCloseAngle()
	return true
}

// readCDATA attempts to read a CDATA section and returns true if
// successful. The opening "<!" has already been consumed.
func (z *Tokenizer) readCDATA() bool {
	const s = "[CDATA["
	for i := 0; i < len(s); i++ {
		c := z.readByte()
		if z.err != nil {
			z.data.End = z.raw.End
			return false
		}
		if c != s[i] {
			// Back up to read the fragment of "[CDATA[" again.
			z.raw.End = z.data.Start
			return false
		}
	}
	z.data.Start = z.raw.End
	brackets := 0
	for {
		c := z.readByte()
		if z.err != nil {
			z.data.End = z.raw.End
			return true
		}
		switch c {
		case ']':
			brackets++
		case '>':
			if brackets >= 2 {
				z.data.End = z.raw.End - len("]]>")
				return true
			}
			brackets = 0
		default:
			brackets = 0
		}
	}
}

// startTagIn returns whether the start tag in z.buf[z.data.Start:z.data.End]
// case-insensitively matches any element of ss.
func (z *Tokenizer) startTagIn(ss ...string) bool {
loop:
	for _, s := range ss {
		if z.data.End-z.data.Start != len(s) {
			continue loop
		}
		for i := 0; i < len(s); i++ {
			c := z.buf[z.data.Start+i]
			if c != s[i] {
				continue loop
			}
		}
		return true
	}
	return false
}

func (z *Tokenizer) hasAttribute(s string) bool {
	for i := len(z.attr) - 1; i >= 0; i-- {
		x := z.attr[i]
		key := z.buf[x[0].Start:x[0].End]
		if string(key) == s {
			return true
		}
	}
	return false
}

// readStartTag reads the next start tag token. The opening "<a" has already
// been consumed, where 'a' means anything in [A-Za-z].
func (z *Tokenizer) readStartTag() TokenType {
	z.readTag(true)
	// Several tags flag the tokenizer's next token as raw.
	c, raw, noExpression := z.buf[z.data.Start], false, false
	switch c {
	case 'i':
		raw = z.startTagIn("iframe")
	case 'n':
		raw = z.startTagIn("noembed", "noframes")
	case 'm':
		noExpression = z.startTagIn("math")
	case 'p':
		raw = z.startTagIn("plaintext")
	case 's':
		raw = z.startTagIn("script", "style")
	case 't':
		raw = z.startTagIn("textarea", "title")
	case 'x':
		raw = z.startTagIn("xmp")
	}
	if !raw {
		raw = z.hasAttribute("is:raw")
	}
	if raw {
		z.rawTag = string(z.buf[z.data.Start:z.data.End])
	}
	if noExpression {
		z.noExpressionTag = string(z.buf[z.data.Start:z.data.End])
		z.openBraceIsExpressionStart = false
	}

	// HTML void tags list: https://www.w3.org/TR/2011/WD-html-markup-20110113/syntax.html#syntax-elements
	// Also look for a self-closing token that’s not in the list (e.g. "<svg><path/></svg>")
	if z.startTagIn("area", "base", "br", "col", "command", "embed", "hr", "img", "input", "keygen", "link", "meta", "param", "source", "track", "wbr") || z.err == nil && z.buf[z.raw.End-2] == '/' {
		// Reset tokenizer state for self-closing elements
		z.rawTag = ""
		return SelfClosingTagToken
	}

	// Handle TypeScript Generics
	if len(z.expressionElementStack) > 0 && len(z.expressionElementStack[len(z.expressionElementStack)-1]) == 0 {
		if z.prevToken.Type == TextToken {
			tag := z.buf[z.data.Start:z.data.End]
			a := atom.Lookup(tag)
			// We can be certain this is a start tag if we match an HTML tag, Fragment, or <>
			if a.String() != "" || bytes.Equal(tag, []byte("Fragment")) || bytes.Equal(tag, []byte{}) {
				return StartTagToken
			}
			text := z.prevToken.Data
			originalLen := len(text)
			// If this "StartTagToken" does not include any spaces between it and the end of the expression
			// we can roughly assume it is a TypeScript generic rather than an element. Rough but it works!
			if len(text) != 0 && len(strings.TrimRightFunc(text, unicode.IsSpace)) == originalLen {
				return TextToken
			}
		}
	}

	return StartTagToken
}

// readUnclosedTag reads up until an unclosed tag is implicitly closed.
// Without this function, the tokenizer could get stuck in infinite loops if a
// user is in the middle of typing
func (z *Tokenizer) readUnclosedTag() bool {
	buf := z.buf[z.data.Start:]
	var close int
	if z.fm == FrontmatterOpen {
		close = strings.Index(string(buf), "---")
		if close != -1 && len(buf) < close {
			buf = buf[0:close]
		}
	} else {
		close = bytes.Index(buf, []byte{'>'})
		if close != -1 && len(buf) < close {
			buf = buf[0:close]
		}
	}
	if close == -1 {
		// We can't find a closing tag...
		for i := 0; i < len(buf); i++ {
			c := z.readByte()
			if z.err != nil {
				z.data.End = z.raw.End
				return true
			}

			switch c {
			case ' ', '\n', '\r', '\t', '\f':
				// Safely read up until a whitespace character
				z.data.End = z.raw.End
				return true
			}
		}
		return false
	}
	return false
}

// readTag reads the next tag token and its attributes. If saveAttr, those
// attributes are saved in z.attr, otherwise z.attr is set to an empty slice.
// The opening "<a" or "</a" has already been consumed, where 'a' means anything
// in [A-Za-z].
func (z *Tokenizer) readTag(saveAttr bool) {
	z.pendingAttrType = QuotedAttribute
	z.attr = z.attr[:0]
	z.attrTypes = z.attrTypes[:0]
	z.attrExpressionStack = 0
	z.attrTemplateLiteralStack = make([]int, 0)
	z.nAttrReturned = 0
	// Read the tag name and attribute key/value pairs.
	z.readTagName()
	if z.skipWhiteSpace(); z.err != nil {
		if z.err == io.EOF {
			start := z.prevToken.Loc.Start
			end := z.data.Start
			z.handler.AppendWarning(&loc.ErrorWithRange{
				Code: loc.WARNING_UNCLOSED_HTML_TAG,
				Text: `Unclosed tag`,
				Range: loc.Range{
					Loc: loc.Loc{Start: start},
					Len: end - start,
				},
			})
		}
		return
	}
	for {
		c := z.readByte()
		if z.err != nil || c == '>' {
			break
		}
		z.raw.End--
		z.readTagAttrKey()
		z.readTagAttrVal()
		// Save pendingAttr if saveAttr and that attribute has a non-empty key.
		if saveAttr && z.pendingAttr[0].Start != z.pendingAttr[0].End {
			z.attr = append(z.attr, z.pendingAttr)
			z.attrTypes = append(z.attrTypes, z.pendingAttrType)

			// Warn for common mistakes
			attr := z.attr[len(z.attr)-1]
			// Possible ...spread attribute without wrapping expression
			if attr[0].End-attr[0].Start > 3 {
				text := string(z.buf[attr[0].Start:attr[0].End])
				if len(strings.TrimSpace(text)) > 3 && strings.TrimSpace(text)[0:3] == "..." {
					z.handler.AppendWarning(&loc.ErrorWithRange{
						Code: loc.WARNING_INVALID_SPREAD,
						Text: fmt.Sprintf(`Invalid spread attribute. Did you mean %s?`, fmt.Sprintf("`{%s}`", text)),
						Range: loc.Range{
							Loc: loc.Loc{Start: attr[0].Start},
							Len: len(text),
						},
					})
				}
			}
		}
		if z.skipWhiteSpace(); z.err != nil {
			break
		}
	}
}

// readTagName sets z.data to the "div" in "<div k=v>". The reader (z.raw.End)
// is positioned such that the first byte of the tag name (the "d" in "<div")
// has already been consumed.
func (z *Tokenizer) readTagName() {
	z.data.Start = z.raw.End - 1
	for {
		c := z.readByte()
		if z.err != nil {
			z.data.End = z.raw.End
			return
		}
		switch c {
		case ' ', '\n', '\r', '\t', '\f':
			z.data.End = z.raw.End - 1
			return
		case '/', '>':
			z.raw.End--
			z.data.End = z.raw.End
			return
		}
	}
}

// readTagAttrKey sets z.pendingAttr[0] to the "k" in "<div k=v>".
// Precondition: z.err == nil.
func (z *Tokenizer) readTagAttrKey() {
	z.pendingAttr[0].Start = z.raw.End
	z.pendingAttrType = QuotedAttribute
	for {
		c := z.readByte()
		if z.err != nil {
			z.pendingAttr[0].End = z.raw.End
			return
		}
		switch c {
		case '{':
			z.pendingAttr[0].Start = z.raw.End
			z.pendingAttrType = ShorthandAttribute
			z.attrExpressionStack = 1
			z.attrTemplateLiteralStack = append(z.attrTemplateLiteralStack, 0)
			z.readTagAttrExpression()
			pendingAttr := z.buf[z.pendingAttr[0].Start:]
			if trimmed := strings.TrimSpace(string(pendingAttr)); len(trimmed) > 3 {
				if trimmed[0:3] == "..." {
					z.pendingAttr[0].Start += strings.Index(string(pendingAttr), "...") + 3
					z.pendingAttrType = SpreadAttribute
				}
			}
			continue
		case ' ', '\n', '\r', '\t', '\f', '/':
			if z.pendingAttrType == SpreadAttribute || z.pendingAttrType == ShorthandAttribute {
				z.pendingAttr[0].End = z.raw.End - 2
			} else {
				z.pendingAttr[0].End = z.raw.End - 1
			}
			return
		case '=', '>':
			z.raw.End--
			if z.pendingAttrType == SpreadAttribute || z.pendingAttrType == ShorthandAttribute {
				z.pendingAttr[0].End = z.raw.End - 1
			} else {
				z.pendingAttr[0].End = z.raw.End
			}
			return
		}
	}
}

// readTagAttrVal sets z.pendingAttr[1] to the "v" in "<div k=v>".
func (z *Tokenizer) readTagAttrVal() {
	z.pendingAttr[1].Start = z.raw.End
	z.pendingAttr[1].End = z.raw.End

	if z.skipWhiteSpace(); z.err != nil {
		return
	}
	c := z.readByte()
	if z.err != nil {
		return
	}
	if c != '=' {
		if z.pendingAttrType == QuotedAttribute {
			z.pendingAttrType = EmptyAttribute
		}

		z.raw.End--
		return
	}
	if z.skipWhiteSpace(); z.err != nil {
		return
	}
	quote := z.readByte()
	if z.err != nil {
		return
	}
	switch quote {

	case '>':
		z.raw.End--
		return

	case '\'', '"':
		z.pendingAttr[1].Start = z.raw.End
		z.pendingAttrType = QuotedAttribute
		for {
			c := z.readByte()
			if z.err != nil {
				if z.err == io.EOF {
					// rescan, closing any potentially unterminated quoted attribute values
					for i := z.pendingAttr[1].Start; i < z.raw.End; i++ {
						c := z.buf[i]
						if unicode.IsSpace(rune(c)) || c == '/' || c == '>' {
							z.pendingAttr[1].End = i
							break
						}
						if i == z.raw.End-1 {
							z.pendingAttr[1].End = i
							break
						}
					}
					z.handler.AppendError(&loc.ErrorWithRange{
						Code: loc.ERROR_UNTERMINATED_STRING,
						Text: `Unterminated quoted attribute`,
						Range: loc.Range{
							Loc: loc.Loc{Start: z.data.Start},
							Len: z.raw.End,
						},
					})
					return
				}
				z.pendingAttr[1].End = z.raw.End
				return
			}
			if c == quote {
				z.pendingAttr[1].End = z.raw.End - 1
				return
			}
		}
	case '`':
		z.pendingAttr[1].Start = z.raw.End
		z.pendingAttrType = TemplateLiteralAttribute
		for {
			c := z.readByte()
			if z.err != nil {
				if z.err == io.EOF {
					// rescan, closing any potentially unterminated attribute values
					for i := z.pendingAttr[1].Start; i < z.raw.End; i++ {
						c := z.buf[i]
						if unicode.IsSpace(rune(c)) || c == '/' || c == '>' {
							z.pendingAttr[1].End = i
							break
						}
						if i == z.raw.End-1 {
							z.pendingAttr[1].End = i
							break
						}
					}
					z.handler.AppendError(&loc.ErrorWithRange{
						Code: loc.ERROR_UNTERMINATED_STRING,
						Text: `Unterminated template literal attribute`,
						Range: loc.Range{
							Loc: loc.Loc{Start: z.data.Start},
							Len: z.raw.End,
						},
					})
					return
				}
				z.pendingAttr[1].End = z.raw.End
				return
			}
			if c == quote {
				z.pendingAttr[1].End = z.raw.End - 1
				return
			}
		}

	case '{':
		z.pendingAttr[1].Start = z.raw.End
		z.pendingAttrType = ExpressionAttribute
		z.attrExpressionStack = 1
		z.attrTemplateLiteralStack = append(z.attrTemplateLiteralStack, 0)
		z.readTagAttrExpression()
		z.pendingAttr[1].End = z.raw.End - 1
		return

	default:
		z.pendingAttr[1].Start = z.raw.End - 1
		z.pendingAttrType = QuotedAttribute

		for {
			c := z.readByte()
			if z.err != nil {
				z.pendingAttr[1].End = z.raw.End
				return
			}
			switch c {
			case ' ', '\n', '\r', '\t', '\f':
				z.pendingAttr[1].End = z.raw.End - 1
				return
			case '>':
				z.raw.End--
				z.pendingAttr[1].End = z.raw.End
				return
			}
		}
	}
}

func (z *Tokenizer) allTagAttrExpressionsClosed() bool {
	for i := len(z.attrTemplateLiteralStack); i > 0; i-- {
		item := z.attrTemplateLiteralStack[i-1]
		if item != 0 {
			return false
		}
	}
	return true
}

func (z *Tokenizer) readTagAttrExpression() {
	if z.err != nil {
		return
	}
	for {
		c := z.readByte()
		if z.err != nil {
			return
		}
		switch c {
		case '`':
			current := 0
			if len(z.attrTemplateLiteralStack) >= z.attrExpressionStack {
				current = z.attrTemplateLiteralStack[z.attrExpressionStack-1]
			}
			if current > 0 {
				z.attrTemplateLiteralStack[z.attrExpressionStack-1]--
			} else {
				z.attrTemplateLiteralStack[z.attrExpressionStack-1]++
			}
		// Handle comments, strings within attrs
		case '/', '"', '\'':
			if z.attrTemplateLiteralStack[z.attrExpressionStack-1] != 0 && c == '/' {
				continue
			}
			inTemplateLiteral := len(z.attrTemplateLiteralStack) >= z.attrExpressionStack && z.attrTemplateLiteralStack[z.attrExpressionStack-1] > 0
			if inTemplateLiteral {
				continue
			}
			end := z.data.End
			if c == '/' {
				// Also stop when we hit a '}' character (end of attribute expression)
				z.readCommentOrRegExp([]byte{'}'})
				// If we exit on a '}', ignore the final character here
				lastChar := z.buf[z.data.End-1 : z.data.End][0]
				if lastChar == '}' {
					z.data.End--
				}
			} else {
				z.readString(c)
			}
			z.raw.End = z.data.End
			z.data.End = end
		case '{':
			previousChar := z.buf[z.raw.End-2]
			inTemplateLiteral := len(z.attrTemplateLiteralStack) >= z.attrExpressionStack && z.attrTemplateLiteralStack[z.attrExpressionStack-1] > 0
			if !inTemplateLiteral || previousChar == '$' {
				z.attrExpressionStack++
				z.attrTemplateLiteralStack = append(z.attrTemplateLiteralStack, 0)
			}
		case '}':
			inTemplateLiteral := len(z.attrTemplateLiteralStack) >= z.attrExpressionStack && z.attrTemplateLiteralStack[z.attrExpressionStack-1] > 0
			if !inTemplateLiteral {
				z.attrExpressionStack--
				if z.attrExpressionStack == 0 && z.allTagAttrExpressionsClosed() {
					return
				}
			}
		}
	}
}

func (z *Tokenizer) Loc() loc.Loc {
	return loc.Loc{Start: z.data.Start}
}

// An expression boundary means the next tokens should be treated as a JS expression
// (_do_ handle strings, comments, regexp, etc) rather than as plain text
func (z *Tokenizer) isAtExpressionBoundary() bool {
	if len(z.expressionStack) == 0 {
		return false
	}
	return len(z.expressionElementStack[len(z.expressionElementStack)-1]) == 0
}

func (z *Tokenizer) trackExpressionElementStack() {
	if len(z.expressionStack) == 0 {
		return
	}
	i := len(z.expressionElementStack) - 1
	if z.tt == StartTagToken {
		z.expressionElementStack[i] = append(z.expressionElementStack[i], string(z.buf[z.data.Start:z.data.End]))
	} else if z.tt == EndTagToken {
		stack := z.expressionElementStack[i]
		if len(stack) > 0 {
			for j := 1; j < len(stack)+1; j++ {
				tok := stack[len(stack)-j]
				if tok == string(z.buf[z.data.Start:z.data.End]) {
					// When stack is balanced, reset `openBraceIsExpressionStart`
					if len(stack) == 1 {
						z.expressionElementStack[i] = make([]string, 0)
						z.openBraceIsExpressionStart = false
					} else {
						z.expressionElementStack[i] = stack[:len(stack)-1]
					}
				}
			}
		}
	} else if z.tt == SelfClosingTagToken {
		stack := z.expressionElementStack[i]
		if len(stack) == 0 {
			// Only switch out of this mode if we're not in an active stack
			z.openBraceIsExpressionStart = false
		}
	}
}

// Next scans the next token and returns its type.
func (z *Tokenizer) Next() TokenType {
	z.prevToken = z.Token()
	z.raw.Start = z.raw.End
	z.data.Start = z.raw.End
	z.data.End = z.raw.End
	defer z.trackExpressionElementStack()

	if z.rawTag != "" {
		if z.rawTag == "plaintext" {
			// Read everything up to EOF.
			for z.err == nil {
				z.readByte()
			}
			z.data.End = z.raw.End
			z.textIsRaw = true
		} else if z.rawTag == "title" || z.rawTag == "textarea" {
			goto raw_with_expression_loop
		} else {
			z.readRawOrRCDATA()
		}
		if z.data.End > z.data.Start {
			z.tt = TextToken
			z.convertNUL = true
			return z.tt
		}
	}
	z.textIsRaw = false
	z.convertNUL = false
	if z.fm != FrontmatterClosed {
		goto frontmatter_loop
	}
	if z.isAtExpressionBoundary() {
		goto expression_loop
	}

loop:
	for {
		c := z.readByte()
		if z.err != nil {
			break loop
		}

		var tokenType TokenType

		if c == '{' || c == '}' {
			if x := z.raw.End - len("{"); z.raw.Start < x {
				z.raw.End = x
				z.data.End = x
				z.tt = TextToken
				return z.tt
			}
			z.raw.End--
			goto expression_loop
		}

		if c == '-' && z.fm != FrontmatterClosed {
			z.raw.End--
			goto frontmatter_loop
		}
		if c != '<' {
			continue loop
		}
		if z.fm == FrontmatterOpen {
			z.raw.End--
			goto frontmatter_loop
		}

		// Check if the '<' we have just read is part of a tag, comment
		// or doctype. If not, it's part of the accumulated text token.
		c = z.readByte()
		if z.err != nil {
			break loop
		}

		z.openBraceIsExpressionStart = z.noExpressionTag == ""

		// Empty <> Fragment start tag
		if c == '>' {
			if x := z.raw.End - len("<>"); z.raw.Start < x {
				z.raw.End = x
				z.data.End = x
				z.tt = TextToken
				return z.tt
			}
			z.tt = StartTagToken
			return z.tt
		}

		switch {
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z':
			tokenType = StartTagToken
		case c == '/':
			tokenType = EndTagToken
		case c == '!' || c == '?':
			// We use CommentToken to mean any of "<!--actual comments-->",
			// "<!DOCTYPE declarations>" and "<?xml processing instructions?>".
			tokenType = CommentToken
		default:
			raw := z.Raw()
			// Error: encountered an attempted use of <> syntax with attributes, like `< slot="named">Hello world!</>`
			if len(raw) > 1 && unicode.IsSpace(rune(raw[0])) {
				element := bytes.Split(z.Buffered(), []byte{'>'})
				incorrect := fmt.Sprintf("< %s>", element[0])
				correct := fmt.Sprintf("<Fragment %s>", element[0])
				z.handler.AppendError(&loc.ErrorWithRange{
					Code:  loc.ERROR_FRAGMENT_SHORTHAND_ATTRS,
					Text:  `Unable to assign attributes when using <> Fragment shorthand syntax!`,
					Range: loc.Range{Loc: loc.Loc{Start: z.raw.End - 2}, Len: 3 + len(element[0])},
					Hint:  fmt.Sprintf("To fix this, please change %s to use the longhand Fragment syntax: %s", incorrect, correct),
				})
			}
			// Reconsume the current character.
			z.raw.End--
			continue
		}

		// We have a non-text token, but we might have accumulated some text
		// before that. If so, we return the text first, and return the non-
		// text token on the subsequent call to Next.
		if x := z.raw.End - len("<a"); z.raw.Start < x {
			z.raw.End = x
			z.data.End = x
			z.tt = TextToken
			return z.tt
		}

		// If necessary, implicitly close an unclosed tag to bail out before
		// an infinite loop occurs. Helpful for IDEs which compile as user types.
		if x := z.readUnclosedTag(); x {
			z.tt = TextToken
			return z.tt
		}

		switch tokenType {
		case StartTagToken:
			// If we see an element before "---", ignore any future "---"
			if z.fm == FrontmatterInitial {
				z.fm = FrontmatterClosed
			}
			z.tt = z.readStartTag()
			return z.tt
		case EndTagToken:
			// If we see an element before "---", ignore any future "---"
			if z.fm == FrontmatterInitial {
				z.fm = FrontmatterClosed
			}

			c = z.readByte()
			if z.err != nil {
				break loop
			}
			if c == '>' {
				z.tt = EndTagToken
				return z.tt
			}
			if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' {
				z.readTag(false)
				tagName := string(z.buf[z.data.Start:z.data.End])
				if tagName == z.noExpressionTag {
					// out of the tag block
					z.noExpressionTag = ""
				}
				if z.err != nil {
					z.tt = ErrorToken
				} else {
					z.tt = EndTagToken
				}
				return z.tt
			}
			z.raw.End--
			z.tt = CommentToken
			return z.tt
		case CommentToken:
			if c == '!' {
				z.tt = z.readMarkupDeclaration()
				return z.tt
			}
			z.raw.End--
			z.readUntilCloseAngle()
			z.tt = CommentToken
			return z.tt
		}
	}
	if z.raw.Start < z.raw.End {
		// We're scanning Text, so open braces should be ignored
		z.openBraceIsExpressionStart = false
		z.data.End = z.raw.End
		z.tt = TextToken
		return z.tt
	}
	z.tt = ErrorToken
	return z.tt

frontmatter_loop:
	for {
		if z.fm == FrontmatterClosed {
			goto loop
		}
		c := z.readByte()
		if z.err != nil {
			break frontmatter_loop
		}

		// handle frontmatter fence
		if c == '-' {
			z.dashCount++ // increase dashCount with each consecutive "-"
		}

		if z.dashCount == 3 {
			switch z.fm {
			case FrontmatterInitial:
				z.fm = FrontmatterOpen
				z.dashCount = 0
				z.data.End = z.raw.End
				z.tt = FrontmatterFenceToken
				z.openBraceIsExpressionStart = false
				return z.tt
			case FrontmatterOpen:
				if z.raw.Start < z.raw.End-len("---") {
					z.data.End = z.raw.End - len("---")
					z.openBraceIsExpressionStart = false
					z.tt = TextToken
					return z.tt
				}
				z.fm = FrontmatterClosed
				z.dashCount = 0
				z.data.End = z.raw.End
				z.tt = FrontmatterFenceToken
				z.openBraceIsExpressionStart = z.noExpressionTag == ""
				return z.tt
			}
		}

		if c == '-' {
			continue frontmatter_loop
		}

		// JS Comment or RegExp
		if c == '/' {
			z.readCommentOrRegExp([]byte{})
			z.tt = TextToken
			z.data.End = z.raw.End
			return z.tt
		}

		// handle string
		if c == '\'' || c == '"' || c == '`' {
			z.readString(c)
			z.tt = TextToken
			z.data.End = z.raw.End
			return z.tt
		}

		s := z.buf[z.raw.Start : z.raw.Start+1][0]

		if s == '<' || s == '{' || s == '}' || c == '<' || c == '{' || c == '}' {
			z.dashCount = 0
			if z.fm == FrontmatterOpen && (s == '<' || c == '<') {
				// Do not support elements inside of frontmatter!
				continue frontmatter_loop
			} else {
				z.raw.End--
				goto loop
			}
		}

		z.dashCount = 0
		continue frontmatter_loop
	}
	z.data.End = z.raw.End

raw_with_expression_loop:
	for {
		c := z.readByte()
		if z.err != nil {
			break raw_with_expression_loop
		}

		// handle string
		if c == '`' {
			z.readString(c)
			z.tt = TextToken
			z.data.End = z.raw.End
			return z.tt
		}

		if c == '{' || c == '}' {
			if x := z.raw.End - len("{"); z.raw.Start < x {
				z.raw.End = x
				z.data.End = x
				z.tt = TextToken
				return z.tt
			}
			z.raw.End--
			goto expression_loop
		}
		if c != '<' {
			continue raw_with_expression_loop
		}
		c = z.readByte()
		if z.err != nil {
			break raw_with_expression_loop
		}
		if c != '/' {
			z.raw.End--
			continue raw_with_expression_loop
		}
		if z.readRawEndTag() || z.err != nil {
			break raw_with_expression_loop
		}
	}
	z.data.End = z.raw.End
	// A textarea's or title's RCDATA can contain escaped entities.
	z.textIsRaw = z.rawTag != "textarea" && z.rawTag != "title"
	z.rawTag = ""

expression_loop:
	for {
		c := z.readByte()
		if z.err != nil {
			break expression_loop
		}

		// JS Comment or RegExp
		if c == '/' {
			boundaryChars := []byte{'{', '}', '\'', '"', '`'}
			z.readCommentOrRegExp(boundaryChars)
			// If we exit on a '}', ignore the final character here
			lastChar := z.buf[z.data.End-1 : z.data.End][0]
			for _, c := range boundaryChars {
				if lastChar == c {
					z.raw.End--
				}
			}
			z.data.End = z.raw.End
			z.tt = TextToken
			return z.tt

		}

		// handle string
		if c == '\'' || c == '"' || c == '`' {
			z.readString(c)
			z.tt = TextToken
			z.data.End = z.raw.End
			return z.tt
		}

		if c == '<' {
			// Check next byte to see if this is an element or a JS expression.
			// Note: this is not a perfect check, just good enough for most cases!
			c1 := z.readByte()
			if z.err != nil {
				break expression_loop
			}
			if unicode.IsSpace(rune(c1)) || unicode.IsNumber(rune(c1)) {
				continue
			}

			// Otherwise, we have an element. Reset pointer and try again.
			z.raw.End -= 2
			z.data.End = z.raw.End
			if z.rawTag != "" {
				goto raw_with_expression_loop
			} else {
				goto loop
			}
		}

		if c != '{' && c != '}' {
			continue expression_loop
		}

		if x := z.raw.End - len("{"); z.raw.Start < x {
			z.raw.End = x
			z.data.End = x
			z.tt = TextToken
			return z.tt
		}

		switch c {
		case '{':
			if z.openBraceIsExpressionStart {
				z.openBraceIsExpressionStart = false
				z.expressionStack = append(z.expressionStack, 0)
				z.expressionElementStack = append(z.expressionElementStack, make([]string, 0))
				z.data.End = z.raw.End - 1
				z.tt = StartExpressionToken
				return z.tt
			} else {
				if len(z.expressionStack) > 0 {
					z.expressionStack[len(z.expressionStack)-1]++
				}
				z.data.End = z.raw.End
				z.tt = TextToken
				return z.tt
			}
		case '}':
			if len(z.expressionStack) == 0 {
				z.data.End = z.raw.End
				z.tt = TextToken
				return z.tt
			}
			z.expressionStack[len(z.expressionStack)-1]--
			if z.expressionStack[len(z.expressionStack)-1] == -1 {
				z.openBraceIsExpressionStart = z.noExpressionTag == ""
				z.expressionStack = z.expressionStack[0 : len(z.expressionStack)-1]
				z.expressionElementStack = z.expressionElementStack[0 : len(z.expressionElementStack)-1]
				z.data.End = z.raw.End
				z.tt = EndExpressionToken
				return z.tt
			}
		}
	}
	if z.raw.Start < z.raw.End {
		z.data.End = z.raw.End
		z.tt = TextToken
		return z.tt
	}
	z.tt = ErrorToken
	return z.tt
}

// Raw returns the unmodified text of the current token. Calling Next, Token,
// Text, TagName or TagAttr may change the contents of the returned slice.
//
// The token stream's raw bytes partition the byte stream (up until an
// ErrorToken). There are no overlaps or gaps between two consecutive token's
// raw bytes. One implication is that the byte offset of the current token is
// the sum of the lengths of all previous tokens' raw bytes.
func (z *Tokenizer) Raw() []byte {
	return z.buf[z.raw.Start:z.raw.End]
}

var (
	nul         = []byte("\x00")
	replacement = []byte("\ufffd")
)

// Text returns the unescaped text of a text, comment or doctype token. The
// contents of the returned slice may change on the next call to Next.
func (z *Tokenizer) Text() []byte {
	switch z.tt {
	case TextToken, CommentToken, DoctypeToken:
		s := z.buf[z.data.Start:z.data.End]
		z.data.Start = z.raw.End
		z.data.End = z.raw.End
		if (z.convertNUL || z.tt == CommentToken) && bytes.Contains(s, nul) {
			s = bytes.Replace(s, nul, replacement, -1)
		}

		// Do not unescape text, leave it raw for the browser
		// if !z.textIsRaw {
		// 	s = unescape(s, false)
		// }
		return s
	}
	return nil
}

// TagName returns the lower-cased name of a tag token (the `img` out of
// `<IMG SRC="foo">`) and whether the tag has attributes.
// The contents of the returned slice may change on the next call to Next.
func (z *Tokenizer) TagName() (name []byte, hasAttr bool) {
	if z.data.Start < z.data.End {
		switch z.tt {
		case StartTagToken, EndTagToken, SelfClosingTagToken:
			s := z.buf[z.data.Start:z.data.End]
			z.data.Start = z.raw.End
			z.data.End = z.raw.End
			return s, z.nAttrReturned < len(z.attr)
		}
	}
	return nil, false
}

// TagAttr returns the lower-cased key and unescaped value of the next unparsed
// attribute for the current tag token and whether there are more attributes.
// The contents of the returned slices may change on the next call to Next.
func (z *Tokenizer) TagAttr() (key []byte, keyLoc loc.Loc, val []byte, valLoc loc.Loc, attrType AttributeType, moreAttr bool) {
	if z.nAttrReturned < len(z.attr) {
		switch z.tt {
		case StartTagToken, SelfClosingTagToken:
			x := z.attr[z.nAttrReturned]
			attrType := z.attrTypes[z.nAttrReturned]
			z.nAttrReturned++
			key = z.buf[x[0].Start:x[0].End]
			val = z.buf[x[1].Start:x[1].End]
			keyLoc := loc.Loc{Start: x[0].Start}
			valLoc := loc.Loc{Start: x[1].Start}

			var attrVal []byte
			if attrType == ExpressionAttribute {
				attrVal = val
			} else {
				attrVal = unescape(val, true)
			}

			return key, keyLoc, attrVal, valLoc, attrType, z.nAttrReturned < len(z.attr)
		}
	}
	return nil, loc.Loc{Start: 0}, nil, loc.Loc{Start: 0}, QuotedAttribute, false
}

// Token returns the current Token. The result's Data and Attr values remain
// valid after subsequent Next calls.
func (z *Tokenizer) Token() Token {
	t := Token{Type: z.tt, Loc: z.Loc()}

	switch z.tt {
	case StartExpressionToken:
		t.Data = "{"
	case EndExpressionToken:
		t.Data = "}"
	case TextToken, CommentToken, DoctypeToken:
		t.Data = string(z.Text())
	case StartTagToken, SelfClosingTagToken, EndTagToken:
		name, moreAttr := z.TagName()
		for moreAttr {
			var key, val []byte
			var keyLoc, valLoc loc.Loc
			var attrType AttributeType
			var attrTokenizer *Tokenizer = nil
			key, keyLoc, val, valLoc, attrType, moreAttr = z.TagAttr()
			t.Attr = append(t.Attr, Attribute{"", atom.String(key), keyLoc, string(val), valLoc, attrTokenizer, attrType})
		}
		if isFragment(string(name)) || isComponent(string(name)) {
			t.DataAtom, t.Data = 0, string(name)
		} else if a := atom.Lookup(name); a != 0 {
			t.DataAtom, t.Data = a, a.String()
		} else {
			t.DataAtom, t.Data = 0, string(name)
		}
	}
	return t
}

// NewTokenizer returns a new HTML Tokenizer for the given Reader.
// The input is assumed to be UTF-8 encoded.
func NewTokenizer(r io.Reader) *Tokenizer {
	return NewTokenizerFragment(r, "")
}

// NewTokenizerFragment returns a new HTML Tokenizer for the given Reader, for
// tokenizing an existing element's InnerHTML fragment. contextTag is that
// element's tag, such as "div" or "iframe".
//
// For example, how the InnerHTML "a<b" is tokenized depends on whether it is
// for a <p> tag or a <script> tag.
//
// The input is assumed to be UTF-8 encoded.
func NewTokenizerFragment(r io.Reader, contextTag string) *Tokenizer {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r)
	z := &Tokenizer{
		r:                          r,
		buf:                        buf.Bytes(),
		fm:                         FrontmatterInitial,
		openBraceIsExpressionStart: true,
	}
	if contextTag != "" {
		switch s := strings.ToLower(contextTag); s {
		case "iframe", "noembed", "noframes", "plaintext", "script", "style", "title", "textarea", "xmp":
			z.rawTag = s
		}
	}
	return z
}
