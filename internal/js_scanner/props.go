package js_scanner

import (
	"fmt"
	"strings"

	"github.com/withastro/compiler/internal/vendored/typescript-go/internals/ast"
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

// sets the Ident field to the Props symbol
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

// Visitor function to look for a
// Props type definition
func propDefVisitor(s *Js_scanner, node *ast.Node) bool {
	if node == nil {
		return true
	}

	// Check for interface declaration: interface Props {...}
	if ast.IsInterfaceDeclaration(node) {
		interfaceDecl := node.AsInterfaceDeclaration()
		if name := interfaceDecl.Name(); name != nil && name.AsIdentifier().Text == propSymbol {
			s.Result.Props.applyFoundIdent()

			if typeParams := interfaceDecl.TypeParameters; typeParams != nil {
				s.Result.Props.populateInfo(typeParams, s.source)
			}
			return true
		}
	}

	// Check for type alias: type Props = {...}
	if ast.IsTypeAliasDeclaration(node) {
		typeAlias := node.AsTypeAliasDeclaration()
		if name := typeAlias.Name(); name != nil && name.AsIdentifier().Text == propSymbol {
			s.Result.Props.applyFoundIdent()

			if typeParams := typeAlias.TypeParameters; typeParams != nil {
				s.Result.Props.populateInfo(typeParams, s.source)
			}
			return true
		}
	}

	return false
}

func importPropsVisitor(s *Js_scanner, node *ast.ImportDeclaration) bool {
	importDecl := node.AsImportDeclaration()
	// if there is a default import or named import, named `Props`
	// we can assume that it is a Props type
	if icn := importDecl.ImportClause; icn != nil {
		importClause := icn.AsImportClause()

		if name := importClause.Name(); name != nil && name.AsIdentifier().Text == propSymbol {
			s.Result.Props.applyFoundIdent()
			return true
		}

		if nb := importClause.NamedBindings; nb != nil && nb.Kind == ast.KindNamedImports {
			namedImports := nb.AsNamedImports()
			for _, element := range namedImports.Elements.Nodes {
				importSpecifier := element.AsImportSpecifier()
				if name := importSpecifier.Name(); name != nil && name.AsIdentifier().Text == propSymbol {
					s.Result.Props.applyFoundIdent()
					return true
				}
			}
		}
	}
	return false
}

func (p *Props) hasIdent() bool {
	return p.Ident != ""
}
