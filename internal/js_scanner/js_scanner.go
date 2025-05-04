package js_scanner

import (
	"bytes"
	"iter"
	"strings"

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
	Props             *Props
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
		Props:       &Props{},
	}

	defer func() {
		if !s.Props.hasIdent() {
			s.Props.Ident = FallbackPropsType
		}
	}()

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
	// lhi - looseHasImport
	lhi := strings.Contains(source, importIdent)
	// lhx -looseHasExport
	lhx := strings.Contains(source, exportIdent)
	// lhp - looseHasProps
	lhp := strings.Contains(source, propSymbol)

	if !lhi && !lhx && !lhp {
		// TODO: make sure it doesn't result to
		// bad sourcemaps
		s.addBody(0, len(source))
		return
	}
	// lhgsp - lhgsp
	lhgsp := strings.Contains(source, gspIdent)

	// use an absolute‐style path for parser
	fileName := "/astro-frontmatter.ts"

	path := tspath.Path(fileName)
	// parse with ESNext + full JSDoc mode
	sf := parser.ParseSourceFile(fileName, path, source, core.ScriptTargetESNext, scanner.JSDocParsingModeParseAll)
	rootNode := sf.AsNode()

	var visitor ast.Visitor = func(n *ast.Node) bool {
		return segmentsVisitor(s, n, lhi, lhx, lhgsp)
	}

	rootNode.ForEachChild(visitor)

	lastChild := sf.Statements.Nodes[len(sf.Statements.Nodes)-1]
	lastChildEnd := lastChild.End()

	if lastChildEnd < len(source) {
		// For some reason, the EOF node isn't included in the AST,
		// because of that, we can't retrieve the trailing trivia
		// This is a workaround to get the trailing trivia
		s.addBody(lastChildEnd, len(source))
	}
}

// only iterate immediate children (top‑level statements)
func segmentsVisitor(s *Js_scanner, n *ast.Node, lhi, lhx, lhgsp bool) bool {
	if n == nil {
		return true
	}

	hasExportMod := ast.HasSyntacticModifier(n, ast.ModifierFlagsExport)
	var isBody bool

	switch {
	case lhi && ast.IsImportDeclaration(n) && n.AsImportDeclaration().ModuleSpecifier != nil:
		s.addHoistedImport(n.Pos(), n.End())
		s.addImportNode(n)
		// visit the import to check for Props
		if !s.Props.hasIdent() {
			importPropsVisitor(s, n.AsImportDeclaration())
		}

	case lhx && (ast.IsExportDeclaration(n) || hasExportMod):
		export := s.addHoistedExport(n.Pos(), n.End())
		if lhgsp && !s.HasGetStaticPaths && hasGetStaticPaths(export) {
			s.HasGetStaticPaths = true
		}

	default:
		isBody = true
		s.addBody(n.Pos(), n.End())
	}

	// look for a Props type/interface definition
	// with or without an export modifier
	if hasExportMod || isBody {
		if !s.Props.hasIdent() {
			propDefVisitor(s, n)
		}
	}

	return false
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

func (s *Js_scanner) NextImportStatement() iter.Seq[ImportStatement] {
	return func(yield func(ImportStatement) bool) {
		for _, node := range s.Imports {

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
				if !yield(ImportStatement{
					Span:      loc.Span{Start: start, End: end},
					Value:     s.source[start:end],
					Specifier: moduleSpecifierString,
				}) {
					return
				}
				continue
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
				if !yield(ImportStatement{
					Span:       loc.Span{Start: start, End: end},
					Value:      s.source[start:end],
					IsType:     importClause.IsTypeOnly,
					Imports:    imports,
					Specifier:  moduleSpecifierString,
					Assertions: assertions,
				}) {
					return
				}
				continue
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

			if !yield(ImportStatement{
				Span:       loc.Span{Start: start, End: end},
				Value:      s.source[start:end],
				IsType:     importClause.IsTypeOnly,
				Imports:    imports,
				Specifier:  moduleSpecifierString,
				Assertions: assertions,
			}) {
				return
			}
		}
	}
}
