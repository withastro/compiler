package transform

import (
	tycho "github.com/snowpackjs/tycho/internal"
)

type TransformOptions struct {
	Scope string
}

func Transform(doc *tycho.Node, opts TransformOptions) {
	walk(doc, func(n *tycho.Node) {
		ScopeElement(n, opts)
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
