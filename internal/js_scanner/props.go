package js_scanner

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/ast"
	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/core"
	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/parser"
	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/scanner"
	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/tspath"
)

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
