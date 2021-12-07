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

		// Special case for import assertions, probably should be tracking that this is inside of an import statement.
		if token == js.IdentifierToken && string(value) == "assert" {
			i += len(value)
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

		// If there are no open pairs and we hit anything other than a comment
		// return our index! This is the first non-exported declaration
		if !openPairs && token != js.CommentToken {
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

type HoistedScripts struct {
	Hoisted [][]byte
	Body    []byte
}

func HoistExports(source []byte) HoistedScripts {
	shouldHoist := hasGetStaticPaths(source)
	if !shouldHoist {
		return HoistedScripts{
			Body: source,
		}
	}

	l := js.NewLexer(parse.NewInputBytes(source))
	i := 0
	pairs := make(map[byte]int)

	// Let's lex the script until we find what we need!
	for {
		token, value := l.Next()

		if token == js.ErrorToken {
			if l.Err() != io.EOF {
				return HoistedScripts{
					Body: source,
				}
			}
			break
		}

		// Common delimeters. Track their length, then skip.
		if token == js.WhitespaceToken || token == js.LineTerminatorToken || token == js.SemicolonToken {
			i += len(value)
			continue
		}

		// Exports should be consumed until all opening braces are closed,
		// a specifier is found, and a line terminator has been found
		if token == js.ExportToken {
			foundGetStaticPaths := false
			foundSemicolonOrLineTerminator := false
			start := i - 1
			i += len(value)
			for {
				next, nextValue := l.Next()
				i += len(nextValue)

				if js.IsIdentifier(next) {
					if !foundGetStaticPaths {
						foundGetStaticPaths = string(nextValue) == "getStaticPaths"
					}
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

				if next == js.ErrorToken {
					return HoistedScripts{
						Body: source,
					}
				}

				if foundGetStaticPaths && foundSemicolonOrLineTerminator && pairs['{'] == 0 && pairs['('] == 0 && pairs['['] == 0 {
					hoisted := make([][]byte, 1)
					hoisted = append(hoisted, source[start:i])
					body := make([]byte, 0)
					body = append(body, source[0:start]...)
					body = append(body, source[i:]...)
					return HoistedScripts{
						Hoisted: hoisted,
						Body:    body,
					}
				}
			}
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

		// Track our current position
		i += len(value)
	}

	// If we haven't found anything... there's nothing to find! Split at the start.
	return HoistedScripts{
		Body: source,
	}
}

func hasGetStaticPaths(source []byte) bool {
	l := js.NewLexer(parse.NewInputBytes(source))
	for {
		token, value := l.Next()
		if token == js.ErrorToken {
			// EOF or other error
			return false
		}
		if token == js.IdentifierToken && string(value) == "getStaticPaths" {
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

type Import struct {
	ExportName string
	LocalName  string
}
type ImportStatement struct {
	Imports   []Import
	Specifier string
}

type ImportState uint32

const (
	ImportDefault ImportState = iota
	ImportNamed
)

func NextImportStatement(source []byte, pos int) (int, ImportStatement) {
	l := js.NewLexer(parse.NewInputBytes(source[pos:]))
	i := pos
	for {
		token, value := l.Next()
		if token == js.ErrorToken {
			// EOF or other error
			return -1, ImportStatement{}
		}
		// Imports should be consumed up until we find a specifier,
		// then we can exit after the following line terminator or semicolon
		if token == js.ImportToken {
			i += len(value)
			foundSpecifier := false
			specifier := ""
			imports := make([]Import, 0)
			importState := ImportDefault
			currImport := Import{}
			for {
				next, nextValue := l.Next()
				i += len(nextValue)

				if !foundSpecifier && next == js.StringToken {
					foundSpecifier = true
					specifier = string(nextValue[1 : len(nextValue)-1])
				}

				if specifier != "" && (next == js.LineTerminatorToken || next == js.SemicolonToken) {
					if currImport.ExportName != "" {
						if currImport.LocalName == "" {
							currImport.LocalName = currImport.ExportName
						}
						imports = append(imports, currImport)
					}
					return i, ImportStatement{
						Imports:   imports,
						Specifier: specifier,
					}
				}

				if next == js.WhitespaceToken {
					continue
				}

				if next == js.OpenBraceToken {
					importState = ImportNamed
				}

				if next == js.CommaToken {
					if currImport.LocalName == "" {
						currImport.LocalName = currImport.ExportName
					}
					imports = append(imports, currImport)
					currImport = Import{}
				}

				if next == js.IdentifierToken {
					if currImport.ExportName != "" {
						currImport.LocalName = string(nextValue)
					} else if importState == ImportNamed {
						currImport.ExportName = string(nextValue)
					} else if importState == ImportDefault {
						currImport.ExportName = "default"
						currImport.LocalName = string(nextValue)
					}
				}

				if next == js.MulToken {
					currImport.ExportName = string(nextValue)
				}

				// if this is import.meta.*, ignore (watch for first dot)
				if next == js.DotToken && len(specifier) == 0 {
					break
				}
			}
		}

		i += len(value)
	}
}
