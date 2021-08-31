package transform

import (
	tycho "github.com/snowpackjs/astro/internal"
	a "golang.org/x/net/html/atom"
)

type TransformOptions struct {
	Scope       string
	Filename    string
	InternalURL string
}

func Transform(doc *tycho.Node, opts TransformOptions) {
	extractScriptsAndStyles(doc)

	if len(doc.Styles) > 0 {
		ScopeStyle(doc.Styles, opts)

		walk(doc, func(n *tycho.Node) {
			ScopeElement(n, opts)
		})
	}

	if len(doc.Scripts) > 0 {
		// fmt.Println("Found scripts!")
	}
}

func extractScriptsAndStyles(doc *tycho.Node) ([]*tycho.Node, []*tycho.Node) {
	scripts := make([]*tycho.Node, 0)
	styles := make([]*tycho.Node, 0)

	walk(doc, func(n *tycho.Node) {
		if n.Type == tycho.ElementNode {
			switch n.DataAtom {
			case a.Script:
				// for _, attr := range n.Attr {
				// 	if attr.Key == "hoist" {
				// 		doc.Scripts = append(doc.Scripts, n)
				// 	}
				// }
				doc.Scripts = append(doc.Scripts, n)
				// Remove local script node
				n.Parent.RemoveChild(n)
			case a.Style:
				for _, attr := range n.Attr {
					if attr.Key == "hoist" {
						doc.Styles = append(doc.Styles, n)
					}
				}
				doc.Styles = append(doc.Styles, n)
				// Remove local style node
				n.Parent.RemoveChild(n)
			}
		}
	})

	return scripts, styles
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
