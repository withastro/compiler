package transform

import (
	tycho "github.com/snowpackjs/astro/internal"
	a "golang.org/x/net/html/atom"
)

type TransformOptions struct {
	Scope string
}

func Transform(doc *tycho.Node, opts TransformOptions) {
	scripts, styles := extractScriptsAndStyles(doc)

	if len(styles) > 0 {
		ScopeStyle(styles, opts)

		walk(doc, func(n *tycho.Node) {
			ScopeElement(n, opts)
		})
	}

	if len(scripts) > 0 {
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
				scripts = append(scripts, n)
			case a.Style:
				styles = append(scripts, n)
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
