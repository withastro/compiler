package js_scanner

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
	"github.com/withastro/compiler/internal/loc"
)

type HoistedScripts struct {
	Hoisted [][]byte
	Body    []byte
}

func HoistExports(source []byte) HoistedScripts {
	shouldHoist := bytes.Contains(source, []byte("export"))
	if !shouldHoist {
		return HoistedScripts{
			Body: source,
		}
	}

	l := js.NewLexer(parse.NewInputBytes(source))
	i := 0
	end := 0

	hoisted := make([][]byte, 1)
	body := make([]byte, 0)
	pairs := make(map[byte]int)

	// Let's lex the script until we find what we need!
outer:
	for {
		token, value := l.Next()

		if token == js.DivToken || token == js.DivEqToken {
			lns := bytes.Split(source[i+1:], []byte{'\n'})
			if bytes.Contains(lns[0], []byte{'/'}) {
				token, value = l.RegExp()
			}
		}

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
			flags := make(map[string]bool, 0)
			foundIdent := false
			foundSemicolonOrLineTerminator := false
			start := 0
			if i > 0 {
				start = i - 1
			}
			i += len(value)
			for {
				next, nextValue := l.Next()
				if next == js.DivToken || next == js.DivEqToken {
					lns := bytes.Split(source[i+1:], []byte{'\n'})
					if bytes.Contains(lns[0], []byte{'/'}) {
						next, nextValue = l.RegExp()
					}
				}
				i += len(nextValue)
				flags[string(nextValue)] = true

				if js.IsIdentifier(next) {
					if isKeyword(nextValue) && next != js.FromToken {
						continue
					}
					if !foundIdent {
						foundIdent = true
					}
					if flags["&"] {
						flags["&"] = false
					}
				} else if next == js.LineTerminatorToken || next == js.SemicolonToken || (next == js.ErrorToken && l.Err() == io.EOF) {
					if (flags["function"] || flags["=>"]) && !flags["{"] {
						continue
					}
					if flags["&"] {
						continue
					}
					foundSemicolonOrLineTerminator = true
				} else if js.IsPunctuator(next) {
					if nextValue[0] == '{' || nextValue[0] == '(' || nextValue[0] == '[' {
						flags[string(nextValue[0])] = true
						pairs[nextValue[0]]++
					} else if nextValue[0] == '}' {
						pairs['{']--
					} else if nextValue[0] == ')' {
						pairs['(']--
					} else if nextValue[0] == ']' {
						pairs['[']--
					}
				}

				if foundIdent && foundSemicolonOrLineTerminator && pairs['{'] == 0 && pairs['('] == 0 && pairs['['] == 0 {
					hoisted = append(hoisted, source[start:i])
					if end < start {
						body = append(body, source[end:start]...)
					}
					end = i
					continue outer
				}

				if next == js.ErrorToken {
					if l.Err() != io.EOF {
						return HoistedScripts{
							Body: source,
						}
					}
					break outer
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

	body = append(body, source[end:]...)

	return HoistedScripts{
		Hoisted: hoisted,
		Body:    body,
	}
}

func isKeyword(value []byte) bool {
	return js.Keywords[string(value)] != 0
}

func HoistImports(source []byte) HoistedScripts {
	imports := make([][]byte, 0)
	body := make([]byte, 0)
	prev := 0
	for i, statement := NextImportStatement(source, 0); i > -1; i, statement = NextImportStatement(source, i) {
		body = append(body, source[prev:statement.Span.Start]...)
		imports = append(imports, statement.Value)
		prev = i
	}
	if prev == 0 {
		return HoistedScripts{Body: source}
	}
	body = append(body, source[prev:]...)
	return HoistedScripts{Hoisted: imports, Body: body}
}

type Props struct {
	Ident     string
	Statement string
	Generics  string
}

func GetPropsType(source []byte) Props {
	defaultPropType := "Record<string, any>"
	ident := defaultPropType
	genericsIdents := make([]string, 0)
	generics := ""
	statement := ""

	if !bytes.Contains(source, []byte("Props")) {
		return Props{
			Ident:     ident,
			Statement: statement,
			Generics:  generics,
		}
	}
	l := js.NewLexer(parse.NewInputBytes(source))
	i := 0
	pairs := make(map[byte]int)
	idents := make([]string, 0)

	start := 0
	end := 0

outer:
	for {
		token, value := l.Next()

		if token == js.DivToken || token == js.DivEqToken {
			lns := bytes.Split(source[i+1:], []byte{'\n'})
			if bytes.Contains(lns[0], []byte{'/'}) {
				token, value = l.RegExp()
			}
		}

		if token == js.ErrorToken {
			if l.Err() != io.EOF {
				return Props{
					Ident: ident,
				}
			}
			break
		}

		// Common delimeters. Track their length, then skip.
		if token == js.WhitespaceToken || token == js.LineTerminatorToken || token == js.SemicolonToken {
			i += len(value)
			continue
		}

		if token == js.ExtendsToken {
			if bytes.Equal(value, []byte("extends")) {
				idents = append(idents, "extends")
			}
			i += len(value)
			continue
		}

		if pairs['{'] == 0 && pairs['('] == 0 && pairs['['] == 0 && pairs['<'] == 1 && token == js.CommaToken {
			idents = make([]string, 0)
			i += len(value)
			continue
		}

		if js.IsIdentifier(token) {
			if isKeyword(value) {
				i += len(value)
				continue
			}
			if pairs['<'] == 1 && pairs['{'] == 0 {
				foundExtends := false
				for _, id := range idents {
					if id == "extends" {
						foundExtends = true
					}
				}
				if !foundExtends {
					genericsIdents = append(genericsIdents, string(value))
				}
				i += len(value)
				continue
			}
			// Note: do not check that `pairs['{'] == 0` to support named imports
			if pairs['('] == 0 && pairs['['] == 0 && string(value) == "Props" {
				ident = "Props"
			}
			idents = append(idents, string(value))
			i += len(value)
			continue
		}

		if bytes.ContainsAny(value, "<>") {
			if len(idents) > 0 && idents[len(idents)-1] == "Props" {
				start = i
				ident = "Props"
				idents = make([]string, 0)
			}
			for _, c := range value {
				if c == '<' {
					pairs['<']++
					i += len(value)
					continue
				}
				if c == '>' {
					pairs['<']--
					if pairs['<'] == 0 {
						end = i
						break outer
					}
				}
			}
		}

		if token == js.QuestionToken || (pairs['{'] == 0 && token == js.ColonToken) {
			idents = make([]string, 0)
			idents = append(idents, "extends")
		}

		// Track opening and closing braces
		if js.IsPunctuator(token) {
			if value[0] == '{' || value[0] == '(' || value[0] == '[' {
				idents = make([]string, 0)
				pairs[value[0]]++
				i += len(value)
				continue
			} else if value[0] == '}' {
				pairs['{']--
				if pairs['<'] == 0 && pairs['{'] == 0 && ident != defaultPropType {
					end = i
					break outer
				}
			} else if value[0] == ')' {
				pairs['(']--
			} else if value[0] == ']' {
				pairs['[']--
			}
		}

		// Track our current position
		i += len(value)
	}
	if len(genericsIdents) > 0 && ident != defaultPropType {
		generics = fmt.Sprintf("<%s>", strings.Join(genericsIdents, ", "))
		statement = strings.TrimSpace(string(source[start:end]))
	}
	return Props{
		Ident:     ident,
		Statement: statement,
		Generics:  generics,
	}
}

func isIdentifier(value []byte) bool {
	valid := true
	for i, b := range value {
		if i == 0 {
			valid = js.IsIdentifierStart([]byte{b})
		} else if i < len(value)-1 {
			valid = js.IsIdentifierContinue([]byte{b})
		} else {
			valid = js.IsIdentifierEnd([]byte{b})
		}
		if !valid {
			break
		}
	}
	return valid
}

func GetObjectKeys(source []byte) [][]byte {
	keys := make([][]byte, 0)
	pairs := make(map[byte]int)

	if source[0] == '{' && source[len(source)-1] == '}' {
		l := js.NewLexer(parse.NewInputBytes(source[1 : len(source)-1]))
		i := 0
		var prev js.TokenType

		for {
			token, value := l.Next()
			openPairs := pairs['{'] > 0 || pairs['('] > 0 || pairs['['] > 0

			if token == js.DivToken || token == js.DivEqToken {
				lns := bytes.Split(source[i+1:], []byte{'\n'})
				if bytes.Contains(lns[0], []byte{'/'}) {
					token, value = l.RegExp()
				}
			}
			i += len(value)

			if token == js.ErrorToken {
				return keys
			}

			if js.IsPunctuator(token) {
				if value[0] == '{' || value[0] == '(' || value[0] == '[' {
					pairs[value[0]]++
					continue
				} else if value[0] == '}' {
					pairs['{']--
				} else if value[0] == ')' {
					pairs['(']--
				} else if value[0] == ']' {
					pairs['[']--
				}
			}

			if prev != js.ColonToken {
				push := func() {
					if token != js.StringToken {
						keys = append(keys, value)
					} else {
						key := value[1 : len(value)-1]
						ident := string(key)
						if !isIdentifier(key) {
							ident = strcase.ToLowerCamel(string(key))
						}
						if string(key) == ident {
							keys = append(keys, []byte(key))
						} else {
							keys = append(keys, []byte(fmt.Sprintf("%s: %s", value, ident)))
						}
					}
				}
				if !openPairs && (token == js.IdentifierToken || token == js.StringToken) {
					push()
				} else if pairs['['] == 1 && token == js.StringToken {
					push()
				}
			}

			if !openPairs && token != js.WhitespaceToken {
				prev = token
			}
		}
	}

	return keys
}

type Import struct {
	IsType     bool
	ExportName string
	LocalName  string
	Assertions string
}

type ImportStatement struct {
	Span       loc.Span
	Value      []byte
	IsType     bool
	Imports    []Import
	Specifier  string
	Assertions string
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

		if token == js.DivToken || token == js.DivEqToken {
			lns := bytes.Split(source[i+1:], []byte{'\n'})
			if bytes.Contains(lns[0], []byte{'/'}) {
				token, value = l.RegExp()
			}
		}

		if token == js.ErrorToken {
			// EOF or other error
			return -1, ImportStatement{}
		}
		// Imports should be consumed up until we find a specifier,
		// then we can exit after the following line terminator or semicolon
		if token == js.ImportToken {
			i += len(value)
			text := []byte(value)
			isType := false
			specifier := ""
			assertion := ""
			foundSpecifier := false
			foundAssertion := false
			imports := make([]Import, 0)
			importState := ImportDefault
			currImport := Import{}
			pairs := make(map[byte]int)
			for {
				next, nextValue := l.Next()
				if next == js.DivToken || next == js.DivEqToken {
					lns := bytes.Split(source[i+1:], []byte{'\n'})
					if bytes.Contains(lns[0], []byte{'/'}) {
						next, nextValue = l.RegExp()
					}
				}
				i += len(nextValue)
				text = append(text, nextValue...)

				if next == js.ErrorToken {
					break
				}

				if next == js.DotToken {
					isMeta := false
					for {
						next, _ := l.Next()
						if next == js.MetaToken {
							isMeta = true
						}
						if next != js.WhitespaceToken && next != js.MetaToken {
							break
						}
					}
					if isMeta {
						continue
					}
				}

				if !foundSpecifier && next == js.StringToken {
					specifier = string(nextValue[1 : len(nextValue)-1])
					foundSpecifier = true
					continue
				}

				if !foundSpecifier && next == js.IdentifierToken && string(nextValue) == "type" {
					isType = true
				}

				if foundSpecifier && (next == js.LineTerminatorToken || next == js.SemicolonToken) && pairs['{'] == 0 && pairs['('] == 0 && pairs['['] == 0 {
					if currImport.ExportName != "" {
						if currImport.LocalName == "" {
							currImport.LocalName = currImport.ExportName
						}
						imports = append(imports, currImport)
					}
					return i, ImportStatement{
						Span:       loc.Span{Start: i - len(text), End: i},
						Value:      text,
						IsType:     isType,
						Imports:    imports,
						Specifier:  specifier,
						Assertions: assertion,
					}
				}

				if next == js.WhitespaceToken {
					continue
				}

				if foundAssertion {
					assertion += string(nextValue)
				}

				if !foundAssertion && next == js.StringToken {
					specifier = string(nextValue[1 : len(nextValue)-1])
					foundSpecifier = true
					continue
				}

				if !foundAssertion && foundSpecifier && next == js.IdentifierToken && string(nextValue) == "assert" {
					foundAssertion = true
					continue
				}

				if !foundAssertion && next == js.OpenBraceToken {
					importState = ImportNamed
				}

				if !foundAssertion && next == js.CommaToken {
					if currImport.LocalName == "" {
						currImport.LocalName = currImport.ExportName
					}
					imports = append(imports, currImport)
					currImport = Import{}
				}

				if !foundAssertion && next == js.IdentifierToken {
					if currImport.ExportName != "" {
						currImport.LocalName = string(nextValue)
					} else if importState == ImportNamed {
						currImport.ExportName = string(nextValue)
					} else if importState == ImportDefault {
						currImport.ExportName = "default"
						currImport.LocalName = string(nextValue)
					}
				}

				if !foundAssertion && next == js.MulToken {
					currImport.ExportName = string(nextValue)
				}

				if js.IsPunctuator(next) {
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

				// do not hoist dynamic imports
				if next == js.OpenParenToken && len(specifier) == 0 {
					break
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
