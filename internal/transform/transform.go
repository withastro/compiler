package transform

import (
	"fmt"
	"strings"

	tycho "github.com/snowpackjs/astro/internal"
	"golang.org/x/net/html/atom"
	a "golang.org/x/net/html/atom"
)

type TransformOptions struct {
	As              string
	Scope           string
	Filename        string
	InternalURL     string
	SourceMap       string
	Site            string
	PreprocessStyle interface{}
}

func Transform(doc *tycho.Node, opts TransformOptions) *tycho.Node {
	InjectDoctype(doc, opts)
	shouldScope := len(doc.Styles) > 0 && ScopeStyle(doc.Styles, opts)
	walk(doc, func(n *tycho.Node) {
		ExtractScript(doc, n)
		if shouldScope {
			ScopeElement(n, opts)
		}
	})
	return doc
}

func InjectDoctype(doc *tycho.Node, opts TransformOptions) {
	if opts.As != "document" {
		return
	}
	var hasDoctype bool
	walk(doc, func(n *tycho.Node) {
		if hasDoctype {
			return
		}
		if n.Type == tycho.DoctypeNode {
			hasDoctype = true
			return
		}
	})

	if hasDoctype {
		return
	}

	doc.InsertBefore(&tycho.Node{
		Type: tycho.DoctypeNode,
		Data: "html",
	}, doc.FirstChild)
}

func ExtractStyles(doc *tycho.Node) {
	walk(doc, func(n *tycho.Node) {
		if n.Type == tycho.ElementNode {
			switch n.DataAtom {
			case a.Style:
				// Do not extract <style> inside of SVGs
				if n.Parent != nil && n.Parent.DataAtom == atom.Svg {
					return
				}
				doc.Styles = append(doc.Styles, n)
				// Remove local style node
				n.Parent.RemoveChild(n)
			}
		}
	})
}

func ExtractScript(doc *tycho.Node, n *tycho.Node) {
	if n.Type == tycho.ElementNode {
		switch n.DataAtom {
		case a.Script:
			// if <script hoist>, hoist to the document root
			if hasTruthyAttr(n, "hoist") {
				doc.Scripts = append(doc.Scripts, n)
				// Remove local script node
				n.Parent.RemoveChild(n)
			}
			// otherwise leave in place
		default:
			if n.Component || n.CustomElement {
				for _, attr := range n.Attr {
					id := n.Data
					if n.CustomElement {
						id = fmt.Sprintf("'%s'", id)
					}

					if strings.HasPrefix(attr.Key, "client:") {
						doc.HydratedComponents = append(doc.HydratedComponents, n)
						pathAttr := tycho.Attribute{
							Key:  "client:component-path",
							Val:  fmt.Sprintf("$$metadata.getPath(%s)", id),
							Type: tycho.ExpressionAttribute,
						}
						n.Attr = append(n.Attr, pathAttr)

						exportAttr := tycho.Attribute{
							Key:  "client:component-export",
							Val:  fmt.Sprintf("$$metadata.getExport(%s)", id),
							Type: tycho.ExpressionAttribute,
						}
						n.Attr = append(n.Attr, exportAttr)
						break
					}
				}
			}
		}
	}
}

func walk(doc *tycho.Node, cb func(*tycho.Node)) {
	var f func(*tycho.Node)
	f = func(n *tycho.Node) {
		cb(n)
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
}
