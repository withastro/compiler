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
	shouldScope := len(doc.Styles) > 0 && ScopeStyle(doc.Styles, opts)
	walk(doc, func(n *tycho.Node) {
		ExtractScript(doc, n)
		AddComponentProps(doc, n)
		if shouldScope {
			ScopeElement(n, opts)
		}
	})

	// Important! Remove scripts from original location *after* walking the doc
	for _, script := range doc.Scripts {
		script.Parent.RemoveChild(script)
	}

	return doc
}

func ExtractStyles(doc *tycho.Node) {
	walk(doc, func(n *tycho.Node) {
		if n.Type == tycho.ElementNode && n.DataAtom == a.Style {
			// Do not extract <style> inside of SVGs
			if n.Parent != nil && n.Parent.DataAtom == atom.Svg {
				return
			}
			// prepend node to maintain authored order
			doc.Styles = append([]*tycho.Node{n}, doc.Styles...)
		}
	})
	// Important! Remove styles from original location *after* walking the doc
	for _, style := range doc.Styles {
		style.Parent.RemoveChild(style)
	}
}

// TODO: cleanup sibling whitespace after removing scripts/styles
// func removeSiblingWhitespace(n *tycho.Node) {
// 	if c := n.NextSibling; c != nil && c.Type == tycho.TextNode {
// 		content := strings.TrimSpace(c.Data)
// 		if len(content) == 0 {
// 			c.Parent.RemoveChild(c)
// 		}
// 	}
// }

func ExtractScript(doc *tycho.Node, n *tycho.Node) {
	if n.Type == tycho.ElementNode && n.DataAtom == a.Script {
		// if <script hoist>, hoist to the document root
		if hasTruthyAttr(n, "hoist") {
			// prepend node to maintain authored order
			doc.Scripts = append([]*tycho.Node{n}, doc.Scripts...)
		}
	}
}

func AddComponentProps(doc *tycho.Node, n *tycho.Node) {
	if n.Type == tycho.ElementNode && (n.Component || n.CustomElement) {
		for _, attr := range n.Attr {
			id := n.Data
			if n.CustomElement {
				id = fmt.Sprintf("'%s'", id)
			}

			if strings.HasPrefix(attr.Key, "client:") {
				if attr.Key == "client:only" {
					doc.Metadata.ClientOnlyComponents = append([]*tycho.Node{n}, doc.ClientOnlyComponents...)
					break
				}
				// prepend node to maintain authored order
				doc.Metadata.HydratedComponents = append([]*tycho.Node{n}, doc.HydratedComponents...)
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
