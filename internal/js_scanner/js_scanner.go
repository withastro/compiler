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

// a reusable container
type Segments struct {
	Data [][]byte  // the raw byte‐chunks
	Locs []loc.Loc // their corresponding locations
}

type (
	BodiesInfo     = Segments
	HoistedScripts = Segments
)

type Js_scanner struct {
	source            []byte
	Imports           []*ast.Node
	ExportsInfo       *HoistedScripts
	ImportsInfo       *HoistedScripts
	Bodies            *BodiesInfo
	HasGetStaticPaths bool
}

func NewScanner(source []byte) *Js_scanner {
	s := &Js_scanner{
		source:      source,
		ImportsInfo: &HoistedScripts{},
		ExportsInfo: &HoistedScripts{},
		Bodies:      &BodiesInfo{},
	}
	if len(bytes.TrimSpace(source)) == 0 {
		return s
	}

	s.scan()
	return s
}

func (s *Js_scanner) addImportNode(node *ast.Node) {
	s.Imports = append(s.Imports, node)
}

func (s *Js_scanner) addHoistedImport(start int, end int) {
	importBody := s.source[start:end]
	s.ImportsInfo.Data = append(s.ImportsInfo.Data, importBody)
	s.ImportsInfo.Locs = append(s.ImportsInfo.Locs, loc.Loc{Start: start})
}

// returns the body of the hoisted export
// to check for getStaticPaths
func (s *Js_scanner) addHoistedExport(start int, end int) []byte {
	exportBody := s.source[start:end]
	s.ExportsInfo.Data = append(s.ExportsInfo.Data, exportBody)
	s.ExportsInfo.Locs = append(s.ExportsInfo.Locs, loc.Loc{Start: start})
	return exportBody
}

func (s *Js_scanner) addBody(start int, end int) {
	body := s.source[start:end]
	s.Bodies.Data = append(s.Bodies.Data, body)
	s.Bodies.Locs = append(s.Bodies.Locs, loc.Loc{Start: start})
}

// TODO: work on the same AST for all the analysis work
func (s *Js_scanner) scan() {
	source := string(s.source)
	looseHasImport := strings.Contains(source, "import")
	looseHasExport := strings.Contains(source, "export")

	if !looseHasImport && !looseHasExport {
		// TODO: make sure it doesn't result to
		// bad sourcemaps
		s.addBody(0, len(source))
		return
	}
	looseHasGetStaticPaths := looseHasGetStaticPaths(s.source)

	// use an absolute‐style path for parser
	fileName := "/astro-frontmatter.ts"

	// start := time.Now()
	path := tspath.Path(fileName)
	// parse with ESNext + full JSDoc mode
	sf := parser.ParseSourceFile(fileName, path, source, core.ScriptTargetESNext, scanner.JSDocParsingModeParseAll)
	rootNode := sf.AsNode()

	// only iterate immediate children (top‑level statements)
	var visitor ast.Visitor = func(n *ast.Node) bool {
		if n == nil {
			return true
		}

		isImport := looseHasImport &&
			ast.IsImportDeclaration(n) && n.AsImportDeclaration().ModuleSpecifier != nil
		isExport := looseHasExport &&
			(ast.IsExportDeclaration(n) || ast.HasSyntacticModifier(n, ast.ModifierFlagsExport))

		switch {
		case isImport:
			s.addHoistedImport(n.Pos(), n.End())
			s.addImportNode(n)

		case isExport:
			export := s.addHoistedExport(n.Pos(), n.End())
			if looseHasGetStaticPaths && !s.HasGetStaticPaths && hasGetStaticPaths(export) {
				s.HasGetStaticPaths = true
			}

		default:
			s.addBody(n.Pos(), n.End())
		}

		return false
	}

	rootNode.ForEachChild(visitor)
}

const GspIdent = "getStaticPaths"

func looseHasGetStaticPaths(source []byte) bool {
	ident := []byte(GspIdent)
	return bytes.Contains(source, ident)
}

func hasGetStaticPaths(exportBody []byte) bool {
	ident := []byte(GspIdent)
	return bytes.Contains(exportBody, ident)
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
