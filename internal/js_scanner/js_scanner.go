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

	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/ast"
	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/core"
	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/parser"
	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/scanner"
	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/tspath"
)

type HoistedScripts struct {
	Hoisted     [][]byte
	HoistedLocs []loc.Loc
	Body        [][]byte
	BodyLocs    []loc.Loc
}

type CollectedImportsExportsAndRemainingNodes struct {
	Imports []*ast.Node
	Exports []*ast.Node
	Remains []*ast.Node
}

func collectImportsExportsAndRemainingNodes(source string) CollectedImportsExportsAndRemainingNodes {
	// use an absolute‐style path for parser
	fileName := "/astro-frontmatter.ts"

	// start := time.Now()
	path := tspath.Path(fileName)
	// parse with ESNext + full JSDoc mode
	sf := parser.ParseSourceFile(fileName, path, source, core.ScriptTargetESNext, scanner.JSDocParsingModeParseAll)
	rootNode := sf.AsNode()

	var imports []*ast.Node
	var exports []*ast.Node
	var remains []*ast.Node

	// only iterate immediate children (top‑level statements)
	var visitor ast.Visitor
	visitor = func(child *ast.Node) bool {
		if child == nil {
			return true
		}

		switch {
		case ast.IsImportDeclaration(child) && child.AsImportDeclaration().ModuleSpecifier != nil:
			// fmt.Printf("Specifier: %s\n", child.AsImportDeclaration().ModuleSpecifier.AsStringLiteral().Text)
			imports = append(imports, child)
		case ast.IsExportDeclaration(child),
			ast.HasSyntacticModifier(child, ast.ModifierFlagsExport):
			exports = append(exports, child)
		default:
			remains = append(remains, child)
		}

		return false
	}

	rootNode.ForEachChild(visitor)

	return CollectedImportsExportsAndRemainingNodes{
		Imports: imports,
		Exports: exports,
		Remains: remains,
	}
}

func HoistExports(source []byte) HoistedScripts {
	var body [][]byte
	var bodyLocs []loc.Loc
	var hoisted [][]byte
	var hoistedLocs []loc.Loc

	importsAndExports := collectImportsExportsAndRemainingNodes(string(source))

	fmt.Printf("Exports count: %d\n", len(importsAndExports.Exports))

	for _, node := range importsAndExports.Exports {
		start := node.Pos()
		end := node.End()
		exportBody := source[start:end]
		hoisted = append(hoisted, exportBody)
		hoistedLocs = append(hoistedLocs, loc.Loc{Start: start})
	}

	for _, node := range importsAndExports.Remains {
		start := node.Pos()
		end := node.End()
		body = append(body, source[start:end])
		bodyLocs = append(bodyLocs, loc.Loc{Start: start})
	}

	return HoistedScripts{
		Body:        body,
		BodyLocs:    bodyLocs,
		Hoisted:     hoisted,
		HoistedLocs: hoistedLocs,
	}
}

func isKeyword(value []byte) bool {
	return js.Keywords[string(value)] != 0
}

func HoistImports(source []byte) HoistedScripts {
	var body [][]byte
	var bodyLocs []loc.Loc
	var hoisted [][]byte
	var hoistedLocs []loc.Loc

	importsAndExports := collectImportsExportsAndRemainingNodes(string(source))

	fmt.Printf("Imports count: %d\n", len(importsAndExports.Imports))

	for _, node := range importsAndExports.Imports {
		start := node.Pos()
		end := node.End()
		importBody := source[start:end]
		hoisted = append(hoisted, importBody)
		hoistedLocs = append(hoistedLocs, loc.Loc{Start: start})
	}

	for _, node := range importsAndExports.Remains {
		start := node.Pos()
		end := node.End()
		body = append(body, source[start:end])
		bodyLocs = append(bodyLocs, loc.Loc{Start: start})
	}

	return HoistedScripts{
		Body:        body,
		BodyLocs:    bodyLocs,
		Hoisted:     hoisted,
		HoistedLocs: hoistedLocs,
	}
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
							keys = append(keys, fmt.Appendf(nil, "%s: %s", value, ident))
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
	importsAndExports := collectImportsExportsAndRemainingNodes(string(source))

	for _, node := range importsAndExports.Imports {
		start := node.Pos()
		end := node.End()
		if start >= pos {
			var imports []Import
			var assertions string
			var importClause *ast.ImportClause
			importDeclaration := node.AsImportDeclaration()
			importClauseNode := importDeclaration.ImportClause
			moduleSpecifier := importDeclaration.ModuleSpecifier.AsStringLiteral()
			moduleSpecifierString := moduleSpecifier.Text

			if importClauseNode == nil {
				return end, ImportStatement{
					Span:      loc.Span{Start: start, End: end},
					Value:     source[start:end],
					Specifier: moduleSpecifierString,
				}
			}

			// Process assertions only if importAttributes is not nil
			if importAttributes := importDeclaration.Attributes; importAttributes != nil {
				attrNode := importAttributes.AsImportAttributes()
				// calculate the length of the leading strip
				// to turn "assert { type: 'json' }" into " assert { type: 'json' }"
				leadingStripLength := (func() int {
					if attrNode.Token == ast.KindWithKeyword {
						return len("with")
					}
					return len("assert")
				})()
				assertionStart := attrNode.Pos() + leadingStripLength + 1
				assertions = string(source[assertionStart:attrNode.End()])
			}

			importClause = importClauseNode.AsImportClause()
			importName := importClause.Name()
			importNamedBindings := importClause.NamedBindings

			if importName != nil {
				localName := importName.AsIdentifier().Text
				imports = append(imports, Import{
					ExportName: "default",
					LocalName:  localName,
				})
			}

			if importNamedBindings == nil {
				return end, ImportStatement{
					Span:       loc.Span{Start: start, End: end},
					Value:      source[start:end],
					IsType:     importClause.IsTypeOnly,
					Imports:    imports,
					Specifier:  moduleSpecifierString,
					Assertions: assertions,
				}
			}

			switch importNamedBindings.Kind {
			case ast.KindNamedImports:
				importSpecifierList := importNamedBindings.AsNamedImports().Elements
				for _, c := range importSpecifierList.Nodes {
					importSpecifier := c.AsImportSpecifier()
					var exportName string
					var localName string

					name := importSpecifier.Name()
					propertyName := importSpecifier.PropertyName

					if name != nil {
						localName = name.AsIdentifier().Text
					}

					if propertyName != nil {
						exportName = propertyName.AsIdentifier().Text
					} else if name != nil {
						exportName = localName
					}

					imports = append(imports, Import{
						ExportName: exportName,
						LocalName:  localName,
					})
				}
			case ast.KindNamespaceImport:
				namespaceImport := importNamedBindings.AsNamespaceImport()
				var localName string

				name := namespaceImport.Name()

				if name != nil {
					localName = name.AsIdentifier().Text
				}
				imports = append(imports, Import{
					ExportName: "*",
					LocalName:  localName,
				})
			}

			return end, ImportStatement{
				Span:       loc.Span{Start: start, End: end},
				Value:      source[start:end],
				IsType:     importClause.IsTypeOnly,
				Imports:    imports,
				Specifier:  moduleSpecifierString,
				Assertions: assertions,
			}
		}
	}

	return -1, ImportStatement{}
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
	fmt.Printf("LocalName: `%s`\nExportName: `%s`\nIsNamespacedComponent: `%t`\nLocalNameEqualsData: `%t`\n", imported.LocalName, imported.ExportName, isNamespacedComponent, localNameEqualsData)
	if isNamespacedComponent || localNameEqualsData {
		var exportName string
		switch {
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
