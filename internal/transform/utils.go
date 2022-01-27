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

func HasAttr(n *astro.Node, key string) bool {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return true
		}
	}
	return false
}

func IsImplictNode(n *astro.Node) bool {
	return HasAttr(n, astro.ImplicitNodeMarker)
}

func IsImplictNodeMarker(attr astro.Attribute) bool {
	return attr.Key == astro.ImplicitNodeMarker
}

func childCount(n *astro.Node) int {
	var i int
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if IsImplictNode(c) {
			continue
		}
		i++
	}
	return i
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
