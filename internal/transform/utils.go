package transform

import (
	astro "github.com/snowpackjs/astro/internal"
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

func hasAttr(n *astro.Node, key string) bool {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return true
		}
	}
	return false
}

func getQuotedAttr(n *astro.Node, key string) string {
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
