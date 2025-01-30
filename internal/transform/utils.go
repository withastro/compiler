package transform

import (
	astro "github.com/withastro/compiler/internal"
	"golang.org/x/net/html/atom"
)

func hasTruthyAttr(n *astro.Node, key string) bool {
	for _, attr := range n.Attr {
		if attr.Key == key &&
			(attr.Type == astro.EmptyAttribute) ||
			(attr.Type == astro.ExpressionAttribute && attr.Val == "true") ||
			(attr.Type == astro.QuotedAttribute && (attr.Val == "" || attr.Val == "true")) {
			return true
		}
	}
	return false
}

func HasSetDirective(n *astro.Node) bool {
	return HasAttr(n, "set:html") || HasAttr(n, "set:text")
}

func HasInlineDirective(n *astro.Node) bool {
	return HasAttr(n, "is:inline")
}

func AttrIndex(n *astro.Node, key string) int {
	for i, attr := range n.Attr {
		if attr.Key == key {
			return i
		}
	}
	return -1
}

func HasAttr(n *astro.Node, key string) bool {
	return AttrIndex(n, key) != -1
}

func GetAttr(n *astro.Node, key string) *astro.Attribute {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return &attr
		}
	}
	return nil
}

func IsHoistable(n *astro.Node, renderScriptEnabled bool) bool {
	parent := n.Closest(func(p *astro.Node) bool {
		return p.DataAtom == atom.Svg || p.DataAtom == atom.Noscript || p.DataAtom == atom.Template
	})

	if renderScriptEnabled && parent != nil && parent.Expression {
		return true
	}

	return parent == nil
}

func IsImplicitNode(n *astro.Node) bool {
	return HasAttr(n, astro.ImplicitNodeMarker)
}

func IsImplicitNodeMarker(attr astro.Attribute) bool {
	return attr.Key == astro.ImplicitNodeMarker
}

func IsTopLevel(n *astro.Node) bool {
	if IsImplicitNode(n) || n.Data == "" {
		return false
	}
	p := n.Parent
	if p == nil {
		return true
	}
	if IsImplicitNode(p) || p.Data == "" {
		return true
	}
	if p.Component {
		return IsTopLevel(p)
	}
	return false
}

func GetQuotedAttr(n *astro.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			if attr.Type == astro.QuotedAttribute {
				return attr.Val
			}
			return ""
		}
	}
	return ""
}
