package js_scanner

import (
	"io"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
)

// An ImportType is the type of import.
type ImportType uint32

const (
	StandardImport ImportType = iota
	DynamicImport
)

type ImportStatement struct {
	Type           ImportType
	Start          int
	End            int
	StatementStart int
	StatementEnd   int
}

var source []byte
var pos int

// This function returns the index at which we should split the frontmatter.
// The first slice contains any top-level imports/exports, which are global.
// The second slice contains any non-exported declarations, which are scoped to the render body
//
// Why use a lexical scan here?
//   1. We can stop lexing as soon as we hit a non-exported token
//   2. Lexing supports malformed modules, they'll throw at runtime instead of compilation
//   3. `tdewolff/parse/v2` doesn't support TypeScript parsing yet, but we can lex it fine
func FindRenderBody(source []byte) int {
	l := js.NewLexer(parse.NewInputBytes(source))
	i := 0
	pairs := make(map[byte]int)

	// Let's lex the script until we find what we need!
	for {
		token, value := l.Next()
		openPairs := pairs['{'] > 0 || pairs['('] > 0 || pairs['['] > 0

		if token == js.ErrorToken {
			if l.Err() != io.EOF {
				return -1
			}
			break
		}

		// Common delimeters. Track their length, then skip.
		if token == js.WhitespaceToken || token == js.LineTerminatorToken || token == js.SemicolonToken {
			i += len(value)
			continue
		}

		// Imports should be consumed up until we find a specifier,
		// then we can exit after the following line terminator or semicolon
		if token == js.ImportToken {
			i += len(value)
			foundSpecifier := false
			for {
				next, nextValue := l.Next()
				i += len(nextValue)
				if next == js.StringToken {
					foundSpecifier = true
				}
				if foundSpecifier && (next == js.LineTerminatorToken || next == js.SemicolonToken) {
					break
				}
			}
			continue
		}

		// Exports should be consumed until all opening braces are closed,
		// a specifier is found, and a line terminator has been found
		if token == js.ExportToken {
			foundIdentifier := false
			foundSemicolonOrLineTerminator := false
			i += len(value)
			for {
				next, nextValue := l.Next()
				i += len(nextValue)
				if js.IsIdentifier(next) {
					foundIdentifier = true
				} else if next == js.LineTerminatorToken || next == js.SemicolonToken {
					foundSemicolonOrLineTerminator = true
				} else if js.IsPunctuator(next) {
					if nextValue[0] == '{' || nextValue[0] == '(' || nextValue[0] == '[' {
						pairs[nextValue[0]]++
					} else if nextValue[0] == '}' {
						pairs['{']--
					} else if nextValue[0] == ')' {
						pairs['(']--
					} else if nextValue[0] == ']' {
						pairs['[']--
					}
				}

				if foundIdentifier && foundSemicolonOrLineTerminator && pairs['{'] == 0 && pairs['('] == 0 && pairs['['] == 0 {
					break
				}
			}
			continue
		}

		// Track opening and closing braces
		if js.IsPunctuator(token) {
			if value[0] == '{' || value[0] == '(' || value[0] == '[' {
				pairs[value[0]]++
				i += len(value)
				continue
			} else if value[0] == '}' {
				pairs['{']--
			} else if value[0] == ')' {
				pairs['(']--
			} else if value[0] == ']' {
				pairs['[']--
			}
		}

		// If there are no open pairs and we hit a reserved word (var/let/const/async/function)
		// return our index! This is the first non-exported declaration
		if !openPairs && js.IsReservedWord(token) {
			return i
		}

		// Track our current position
		i += len(value)
	}

	// If we haven't found anything... there's nothing to find! Split at the start.
	return i
}

func HasExports(source []byte) bool {
	l := js.NewLexer(parse.NewInputBytes(source))
	for {
		token, _ := l.Next()
		if token == js.ErrorToken {
			// EOF or other error
			return false
		}
		if token == js.ExportToken {
			return true
		}
	}
}

func AccessesPrivateVars(source []byte) bool {
	l := js.NewLexer(parse.NewInputBytes(source))
	for {
		token, value := l.Next()
		if token == js.ErrorToken {
			// EOF or other error
			return false
		}
		if js.IsIdentifier(token) && len(value) > 1 && value[0] == '$' && value[1] == '$' {
			return true
		}
	}
}

// TODO: refactor to use lexer!
func NextImportSpecifier(_source []byte, _pos int) (int, string) {
	source = _source
	pos = _pos
	inImport := false
	var cont bool
	var start int
	end := 0

MainLoop:
	for ; pos < len(source)-1; pos++ {
		c := readCommentWhitespace(true)

		if inImport {
			if c == '"' || c == '\'' {
				pos++
				start = pos
				readString(start, c)
				end = pos

				// Continue the loop
				cont = true
				break MainLoop
			}
		} else {
			switch true {
			case c == 'i':
				if isKeywordStart() && str_eq6('i', 'm', 'p', 'o', 'r', 't') {
					pos += 6
					inImport = true
					continue
				}
			case c == '/':
				if str_eq2('/', '/') {
					readLineComment()
					continue
				} else if str_eq2('/', '*') {
					readBlockComment(true)
					continue
				}
			}
		}
	}

	if cont {
		specifier := source[start:end]
		return pos, string(specifier)
	} else {
		return -1, ""
	}
}

// The following utilities are adapted from https://github.com/guybedford/es-module-lexer
// Released under the MIT License (C) 2018-2021 Guy Bedford

// Note: non-asii BR and whitespace checks omitted for perf / footprint
// if there is a significant user need this can be reconsidered
func isBr(c byte) bool {
	return c == '\r' || c == '\n'
}

func isWsNotBr(c byte) bool {
	return c == 9 || c == 11 || c == 12 || c == 32 || c == 160
}

func isBrOrWs(c byte) bool {
	return c > 8 && c < 14 || c == 32 || c == 160
}

func isBrOrWsOrPunctuatorNotDot(c byte) bool {
	return c > 8 && c < 14 || c == 32 || c == 160 || isPunctuator(c) && c != '.'
}

func isPunctuator(ch byte) bool {
	// 23 possible punctuator endings: !%&()*+,-./:;<=>?[]^{}|~
	return ch == '!' || ch == '%' || ch == '&' ||
		ch > 39 && ch < 48 || ch > 57 && ch < 64 ||
		ch == '[' || ch == ']' || ch == '^' ||
		ch > 122 && ch < 127
}

func str_eq2(c1 byte, c2 byte) bool {
	return len(source[pos:]) >= 2 && source[pos+1] == c2 && source[pos] == c1
}

func str_eq6(c1 byte, c2 byte, c3 byte, c4 byte, c5 byte, c6 byte) bool {
	return len(source[pos:]) >= 6 && source[pos+5] == c6 && source[pos+4] == c5 && source[pos+3] == c4 && source[pos+2] == c3 && source[pos+1] == c2 && source[pos] == c1
}

func isKeywordStart() bool {
	return isBrOrWsOrPunctuatorNotDot(source[pos-1])
}

func readBlockComment(br bool) {
	pos++
	for ; pos < len(source)-1; pos++ {
		c := source[pos]
		if !br && isBr(c) {
			return
		}
		if c == '*' && source[pos+1] == '/' {
			pos++
			return
		}
	}
}

func readLineComment() {
	for ; pos < len(source)-1; pos++ {
		c := source[pos]
		if c == '\n' || c == '\r' {
			return
		}
	}
}

func readCommentWhitespace(br bool) byte {
	var c byte
	for ; pos < len(source)-1; pos++ {
		c = source[pos]
		switch true {
		case c == '/':
			if str_eq2('/', '/') {
				readLineComment()
				continue
			} else if str_eq2('/', '*') {
				readBlockComment(true)
				continue
			} else {
				return c
			}
		case (br && !isBrOrWs(c)):
			return c
		case (!br && !isWsNotBr(c)):
			return c
		}
	}
	return c
}

func readString(start int, quoteChar byte) {
	var c byte

MainLoop:
	for ; pos < len(source)-1; pos++ {
		c = source[pos]
		switch true {
		case c == '\\':
			pos++
			continue
		case c == quoteChar:
			break MainLoop
		}
	}
}
