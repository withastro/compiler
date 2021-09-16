package transform

import (
	tycho "github.com/snowpackjs/astro/internal"
	a "golang.org/x/net/html/atom"
)

type TransformOptions struct {
	As          string
	Scope       string
	Filename    string
	InternalURL string
	SourceMap   string
}

func Transform(doc *tycho.Node, opts TransformOptions) {
	extractScriptsAndStyles(doc)

	if len(doc.Styles) > 0 {
		if shouldScope := ScopeStyle(doc.Styles, opts); shouldScope {
			walk(doc, func(n *tycho.Node) {
				ScopeElement(n, opts)
			})
		}
	}
}

func extractScriptsAndStyles(doc *tycho.Node) {
	walk(doc, func(n *tycho.Node) {
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
			case a.Style:
				doc.Styles = append(doc.Styles, n)
				// Remove local style node
				n.Parent.RemoveChild(n)
			}
		}
	})
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
