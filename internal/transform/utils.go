package transform

import (
	astro "github.com/withastro/compiler/internal"
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

func HasAttr(n *astro.Node, key string) bool {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return true
		}
	}
	return false
}

func GetAttr(n *astro.Node, key string) *astro.Attribute {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return &attr
		}
	}
	return nil
}

func IsImplictNode(n *astro.Node) bool {
	return HasAttr(n, astro.ImplicitNodeMarker)
}

func IsImplictNodeMarker(attr astro.Attribute) bool {
	return attr.Key == astro.ImplicitNodeMarker
}

func IsTopLevel(n *astro.Node) bool {
	p := n.Parent
	if p == nil {
		return true
	}
	if IsImplictNode(p) || p.Data == "" {
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
