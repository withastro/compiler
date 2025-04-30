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

type Js_scanner struct {
	source  []byte
	Imports []*ast.Node
	Exports []*ast.Node
	Remains []*ast.Node
}

func NewScanner(source []byte) *Js_scanner {
	if len(bytes.TrimSpace(source)) == 0 {
		return &Js_scanner{}
	}
	importsAndExports := collectImportsExportsAndRemainingNodes(string(source))
	return &Js_scanner{
		source:  source,
		Imports: importsAndExports.Imports,
		Exports: importsAndExports.Exports,
		Remains: importsAndExports.Remains,
	}
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

func (s *Js_scanner) HoistExports() HoistedScripts {
	var body [][]byte
	var bodyLocs []loc.Loc
	var hoisted [][]byte
	var hoistedLocs []loc.Loc

	for _, node := range s.Exports {
		start := node.Pos()
		end := node.End()
		exportBody := s.source[start:end]
		hoisted = append(hoisted, exportBody)
		hoistedLocs = append(hoistedLocs, loc.Loc{Start: start})
	}

	for _, node := range s.Remains {
		start := node.Pos()
		end := node.End()
		body = append(body, s.source[start:end])
		bodyLocs = append(bodyLocs, loc.Loc{Start: start})
	}

	return HoistedScripts{
		Body:        body,
		BodyLocs:    bodyLocs,
		Hoisted:     hoisted,
		HoistedLocs: hoistedLocs,
	}
}

func (s *Js_scanner) HoistImports() HoistedScripts {
	var body [][]byte
	var bodyLocs []loc.Loc
	var hoisted [][]byte
	var hoistedLocs []loc.Loc

	for _, node := range s.Imports {
		start := node.Pos()
		end := node.End()
		importBody := s.source[start:end]
		hoisted = append(hoisted, importBody)
		hoistedLocs = append(hoistedLocs, loc.Loc{Start: start})
	}

	for _, node := range s.Remains {
		start := node.Pos()
		end := node.End()
		body = append(body, s.source[start:end])
		bodyLocs = append(bodyLocs, loc.Loc{Start: start})
	}

	return HoistedScripts{
		Body:        body,
		BodyLocs:    bodyLocs,
		Hoisted:     hoisted,
		HoistedLocs: hoistedLocs,
	}
}

func (s *Js_scanner) HasGetStaticPaths() bool {
	ident := []byte("getStaticPaths")
	if !bytes.Contains(s.source, ident) {
		return false
	}

	exports := s.HoistExports()
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
	FallbackPropsType = "Record<string, any>"
	propSymbol        = "Props"
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

func (s *Js_scanner) GetPropsType() Props {
	// If source doesn't contain "Props"
	// return default Props type
	if !bytes.Contains(s.source, []byte(propSymbol)) {
		return Props{
			Ident: FallbackPropsType,
		}
	}

	// Use an absolute-style path for parser
	fileName := "/astro-frontmatter.ts"
	path := tspath.Path(fileName)

	// Parse with ESNext + full JSDoc mode
	sf := parser.ParseSourceFile(fileName, path, string(s.source), core.ScriptTargetESNext, scanner.JSDocParsingModeParseAll)
	rootNode := sf.AsNode()

	var propsType Props

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
					propsType.populateInfo(typeParams, s.source)
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
					propsType.populateInfo(typeParams, s.source)
				}
				return true
			}
		}

		return false
	}

	rootNode.ForEachChild(visitor)

	// look for Props type imports if we haven't
	// found the Props type in the frontmatter yet
	if propsType.Ident == "" {
		// now look for the import
		for _, node := range s.Imports {
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

					if importClause.NamedBindings != nil && importClause.NamedBindings.Kind == ast.KindNamedImports {
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

	// fallback to default Props type
	if propsType.Ident == "" {
		propsType.Ident = FallbackPropsType
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

func (s *Js_scanner) NextImportStatement(idx int) (int, ImportStatement) {
	if len(s.Imports) == 0 || idx >= len(s.Imports) || idx < 0 {
		return -1, ImportStatement{}
	}

	node := s.Imports[idx]
	// increment the index to the next import
	idx++

	start := node.Pos()
	end := node.End()

	var imports []Import
	var assertions string
	var importClause *ast.ImportClause

	importDeclaration := node.AsImportDeclaration()
	importClauseNode := importDeclaration.ImportClause
	moduleSpecifier := importDeclaration.ModuleSpecifier.AsStringLiteral()
	moduleSpecifierString := moduleSpecifier.Text

	if importClauseNode == nil {
		return idx, ImportStatement{
			Span:      loc.Span{Start: start, End: end},
			Value:     s.source[start:end],
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
		assertions = string(s.source[assertionStart:attrNode.End()])
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
		return idx, ImportStatement{
			Span:       loc.Span{Start: start, End: end},
			Value:      s.source[start:end],
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

	return idx, ImportStatement{
		Span:       loc.Span{Start: start, End: end},
		Value:      s.source[start:end],
		IsType:     importClause.IsTypeOnly,
		Imports:    imports,
		Specifier:  moduleSpecifierString,
		Assertions: assertions,
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

	if !isNamespacedComponent && !localNameEqualsData {
		return "", false
	}

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
