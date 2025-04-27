package js_scanner

import (
	"bytes"
	"fmt"
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

// TODO: work on the same AST for all the analysis work
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

func HoistImports(source []byte) HoistedScripts {
	var body [][]byte
	var bodyLocs []loc.Loc
	var hoisted [][]byte
	var hoistedLocs []loc.Loc

	importsAndExports := collectImportsExportsAndRemainingNodes(string(source))

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

func (p *Props) populateInfo(typeParams *ast.NodeList, source []byte) {
	if len(typeParams.Nodes) > 0 {
		p.Statement, p.Generics = getPropsInfo(typeParams, source)
	}
}

// applyFoundIdent sets the Ident
// field to the default Props type name
func (p *Props) applyFoundIdent() {
	p.Ident = propSymbol
}

const (
	AbsentPropType = "Record<string, any>"
	propSymbol     = "Props"
)

func getPropsInfo(typeParams *ast.NodeList, source []byte) (statement, generics string) {
	// Extract generics if present
	firstTypeParam := typeParams.Nodes[0].AsTypeParameter()
	lastTypeParam := typeParams.Nodes[len(typeParams.Nodes)-1].AsTypeParameter()
	statement = fmt.Sprintf("<%s>", source[firstTypeParam.Pos():lastTypeParam.End()])

	genericsList := make([]string, 0, len(typeParams.Nodes))
	for _, param := range typeParams.Nodes {
		typeParam := param.AsTypeParameter()
		genericsList = append(genericsList, typeParam.Name().AsIdentifier().Text)
	}
	generics = fmt.Sprintf("<%s>", strings.Join(genericsList, ", "))
	return
}

func GetPropsType(source []byte) Props {
	// If source doesn't contain "Props"
	// return default Props type
	if !bytes.Contains(source, []byte(propSymbol)) {
		return Props{
			Ident: AbsentPropType,
		}
	}

	// Use an absolute-style path for parser
	fileName := "/astro-frontmatter.ts"
	path := tspath.Path(fileName)

	// Parse with ESNext + full JSDoc mode
	sf := parser.ParseSourceFile(fileName, path, string(source), core.ScriptTargetESNext, scanner.JSDocParsingModeParseAll)
	rootNode := sf.AsNode()

	var propsType Props
	propsType.Ident = AbsentPropType

	// Visitor function to find Props type
	var visitor ast.Visitor
	visitor = func(node *ast.Node) bool {
		if node == nil {
			return true
		}

		// Check for interface declaration: interface Props {...}
		if ast.IsInterfaceDeclaration(node) {
			interfaceDecl := node.AsInterfaceDeclaration()
			if interfaceDecl.Name() != nil && interfaceDecl.Name().AsIdentifier().Text == propSymbol {
				propsType.applyFoundIdent()

				if interfaceDecl.TypeParameters != nil {
					typeParams := interfaceDecl.TypeParameters
					propsType.populateInfo(typeParams, source)
				}
				return true
			}
		}

		// Check for type alias: type Props = {...}
		if ast.IsTypeAliasDeclaration(node) {
			typeAlias := node.AsTypeAliasDeclaration()
			if typeAlias.Name() != nil && typeAlias.Name().AsIdentifier().Text == propSymbol {
				propsType.applyFoundIdent()

				if typeAlias.TypeParameters != nil {
					typeParams := typeAlias.TypeParameters
					propsType.populateInfo(typeParams, source)
				}
				return true
			}
		}

		return false
	}

	rootNode.ForEachChild(visitor)

	if propsType.Ident == AbsentPropType {
		// now look for the import
		imports := collectImportsExportsAndRemainingNodes(string(source)).Imports
		for _, node := range imports {
			if ast.IsImportDeclaration(node) {
				importDecl := node.AsImportDeclaration()
				// if there is a default import or named import, named `Props`
				// we can assume that it is a Props type
				if importDecl.ImportClause != nil {
					importClause := importDecl.ImportClause.AsImportClause()

					if importClause.Name() != nil && importClause.Name().AsIdentifier().Text == propSymbol {
						propsType.applyFoundIdent()
						break
					}

					if importClause.NamedBindings != nil {
						if importClause.NamedBindings.Kind == ast.KindNamedImports {
							namedImports := importClause.NamedBindings.AsNamedImports()
							for _, element := range namedImports.Elements.Nodes {
								importSpecifier := element.AsImportSpecifier()
								if importSpecifier.Name() != nil && importSpecifier.Name().AsIdentifier().Text == propSymbol {
									propsType.applyFoundIdent()
									break
								}
							}
						}
					}
				}
			}
		}
	}

	return propsType
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
