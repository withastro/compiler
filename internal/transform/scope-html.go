package transform

import (
	"strings"

	tycho "github.com/snowpackjs/astro/internal"
)

func ScopeElement(n *tycho.Node, opts TransformOptions) {
	if n.Type == tycho.ElementNode {
		if _, noScope := NeverScopedElements[n.Data]; !noScope {
			injectScopedClass(n, opts)
		}
	}
}

var NeverScopedElements map[string]bool = map[string]bool{
	"Fragment": true,
	"base":     true,
	"body":     true,
	"font":     true,
	"frame":    true,
	"frameset": true,
	"head":     true,
	"html":     true,
	"link":     true,
	"meta":     true,
	"noframes": true,
	"noscript": true,
	"script":   true,
	"style":    true,
	"title":    true,
}

func injectScopedClass(n *tycho.Node, opts TransformOptions) {
	for i, attr := range n.Attr {
		// If we find an existing class attribute, append the scoped class
		if attr.Key == "class" || (n.Component && attr.Key == "className") {
			switch attr.Type {
			case tycho.ShorthandAttribute:
				if n.Component {
					attr.Val = attr.Key + ` + " astro-` + opts.Scope + `"`
					attr.Type = tycho.ExpressionAttribute
					n.Attr[i] = attr
					return
				}
			case tycho.EmptyAttribute:
				// instead of an empty string
				attr.Type = tycho.QuotedAttribute
				attr.Val = "astro-" + opts.Scope
				n.Attr[i] = attr
				return
			case tycho.QuotedAttribute, tycho.TemplateLiteralAttribute:
				// as a plain string
				attr.Val = strings.TrimSpace(attr.Val) + " astro-" + opts.Scope
				n.Attr[i] = attr
				return
			case tycho.ExpressionAttribute:
				// as an expression
				attr.Val = attr.Val + ` + " astro-` + opts.Scope + `"`
				n.Attr[i] = attr
				return
			}
		}
	}
	// If we didn't find an existing class attribute, let's add one
	n.Attr = append(n.Attr, tycho.Attribute{
		Key: "class",
		Val: "astro-" + opts.Scope,
	})
}
