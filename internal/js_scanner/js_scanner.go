package js_scanner

import (
	"io"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
)

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
	pairs := make(map[byte]int, 0)

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
	return 0
}
