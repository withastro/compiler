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
	"github.com/withastro/compiler/ts_parser"
)

type HoistedScripts struct {
	Hoisted     [][]byte
	HoistedLocs []loc.Loc
	Body        [][]byte
	BodyLocs    []loc.Loc
}

func isKeyword(value []byte) bool {
	return js.Keywords[string(value)] != 0
}

type (
	importItems []ts_parser.BodyItem
	exportItems []ts_parser.BodyItem
	otherItems  []ts_parser.BodyItem
)

func CollectImportsAndExports(source []byte) (importItems, exportItems, otherItems) {
	tsParser, _ := ts_parser.GetParser()
	// TODO(mk): revisit where the cleanup should be called

	imports := make([]ts_parser.BodyItem, 0)
	exports := make([]ts_parser.BodyItem, 0)
	others := make([]ts_parser.BodyItem, 0)

	tsAst := tsParser(string(source))

	for _, bodyItem := range tsAst.Body {
		switch bodyItem.Type {
		case ts_parser.ExportNamedDeclaration, ts_parser.ExportAllDeclaration, ts_parser.ExportDefaultDeclaration:
			exports = append(exports, bodyItem)
		case ts_parser.ImportDeclaration:
			imports = append(imports, bodyItem)
		default:
			others = append(others, bodyItem)
		}
	}
	return imports, exports, others
}

func HoistExports(source []byte) HoistedScripts {
	body := make([][]byte, 0)
	bodyLocs := make([]loc.Loc, 0)
	cursor := 0
	shouldHoist := bytes.Contains(source, []byte("export"))
	if !shouldHoist {
		bodyLocs = append(bodyLocs, loc.Loc{Start: 0})
		body = append(body, source)
		return HoistedScripts{Body: body, BodyLocs: bodyLocs}
	}

	exports := make([][]byte, 0)
	exportLocs := make([]loc.Loc, 0)
	_, exportsAst, bodiesAst := CollectImportsAndExports(source)
	for _, exportAst := range exportsAst {
		exports = append(exports, source[exportAst.Start:exportAst.End])
		exportLocs = append(exportLocs, loc.Loc{Start: exportAst.Start})
		cursor = exportAst.End
	}
	if cursor == 0 {
		bodyLocs = append(bodyLocs, loc.Loc{Start: 0})
		body = append(body, source)
		return HoistedScripts{Body: body, BodyLocs: bodyLocs}
	}

	for _, bodyAst := range bodiesAst {
		body = append(body, source[bodyAst.Start:bodyAst.End])
		bodyLocs = append(bodyLocs, loc.Loc{Start: bodyAst.Start})
	}
	return HoistedScripts{Hoisted: exports, HoistedLocs: exportLocs, Body: body, BodyLocs: bodyLocs}
}

func HoistImports(source []byte) HoistedScripts {
	body := make([][]byte, 0)
	bodyLocs := make([]loc.Loc, 0)
	cursor := 0
	shouldHoist := bytes.Contains(source, []byte("import"))
	if !shouldHoist {
		bodyLocs = append(bodyLocs, loc.Loc{Start: 0})
		body = append(body, source)
		return HoistedScripts{Body: body, BodyLocs: bodyLocs}
	}
	imports := make([][]byte, 0)
	importLocs := make([]loc.Loc, 0)
	importsAst, _, _ := CollectImportsAndExports(source)
	for _, importAst := range importsAst {
		bodyLocs = append(bodyLocs, loc.Loc{Start: cursor})
		body = append(body, source[cursor:importAst.Start])
		imports = append(imports, source[importAst.Start:importAst.End])
		importLocs = append(importLocs, loc.Loc{Start: importAst.Start})
		cursor = importAst.End
	}
	if cursor == 0 {
		bodyLocs = append(bodyLocs, loc.Loc{Start: 0})
		body = append(body, source)
		return HoistedScripts{Body: body, BodyLocs: bodyLocs}
	}
	bodyLocs = append(bodyLocs, loc.Loc{Start: cursor})
	body = append(body, source[cursor:])
	return HoistedScripts{Hoisted: imports, HoistedLocs: importLocs, Body: body, BodyLocs: bodyLocs}
}

func HasGetStaticPaths(source []byte) bool {
	ident := []byte("getStaticPaths")
	if !bytes.Contains(source, ident) {
		return false
	}

	exports := HoistExports(source)
	for _, statement := range exports.Hoisted {
		if bytes.Contains(statement, ident) {
			return true
		}
	}
	return false
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
			if len(source) > i {
				lns := bytes.Split(source[i+1:], []byte{'\n'})
				if bytes.Contains(lns[0], []byte{'/'}) {
					token, value = l.RegExp()
				}
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

		// Common delimiters. Track their length, then skip.
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
				// fix(#814): fix Props detection when using `{ Props as SomethingElse }`
				if ident == "Props" && string(value) == "as" {
					start = 0
					ident = defaultPropType
					idents = make([]string, 0)
				}
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
						// Important: only break out if we've already found `Props`!
						if ident != defaultPropType {
							break outer
						} else {
							continue
						}
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
	if start > 0 && len(genericsIdents) > 0 && ident != defaultPropType {
		generics = fmt.Sprintf("<%s>", strings.Join(genericsIdents, ", "))
		statement = strings.TrimSpace(string(source[start:end]))
	}

	return Props{
		Ident:     ident,
		Statement: statement,
		Generics:  generics,
	}
}

func IsIdentifier(value []byte) bool {
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
						if !IsIdentifier(key) {
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
	ExportName string
	LocalName  string
	Assertions string
	IsType     bool
}

type ImportStatement struct {
	Specifier  string
	Assertions string
	Value      []byte
	Imports    []Import
	Span       loc.Span
	IsType     bool
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

		if len(source) > i && token == js.DivToken || token == js.DivEqToken {
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
				if len(source) > i && (next == js.DivToken || next == js.DivEqToken) {
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
					if len(nextValue) > 1 {
						specifier = string(nextValue[1 : len(nextValue)-1])
						foundSpecifier = true
					}
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

				// do not hoist `{ import: "value" }`
				if next == js.ColonToken && len(specifier) == 0 {
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

/*
Determines the export name of a component, i.e. the object path to which
we can access the module, if it were imported using a dynamic import (`import()`)

Returns the export name and a boolean indicating whether
the component is imported AND used in the template.
*/
func ExtractComponentExportName(data string, imported Import) (string, bool) {
	namespacePrefix := fmt.Sprintf("%s.", imported.LocalName)
	isNamespacedComponent := strings.Contains(data, ".") && strings.HasPrefix(data, namespacePrefix)
	localNameEqualsData := imported.LocalName == data
	if isNamespacedComponent || localNameEqualsData {
		var exportName string
		switch true {
		case localNameEqualsData:
			exportName = imported.ExportName
		case imported.ExportName == "*":
			// matched a namespaced import
			exportName = strings.Replace(data, namespacePrefix, "", 1)
		case imported.ExportName == "default":
			// matched a default import
			exportName = strings.Replace(data, imported.LocalName, "default", 1)
		default:
			// matched a named import
			exportName = data
		}
		return exportName, true
	}
	return "", false
}
