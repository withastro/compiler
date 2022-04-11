package transform

import (
	"fmt"
	"strings"

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

func IsImplictNode(n *astro.Node) bool {
	return HasAttr(n, astro.ImplicitNodeMarker)
}

func IsImplictNodeMarker(attr astro.Attribute) bool {
	return attr.Key == astro.ImplicitNodeMarker
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

func GetAttr(n *astro.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key != key {
			continue
		}
		switch a.Type {
		case astro.QuotedAttribute:
			return fmt.Sprintf(`"%s"`, a.Val)
		case astro.EmptyAttribute:
			return `true`
		case astro.ExpressionAttribute:
			if a.Val == "" {
				return `(void 0)`
			}
			return fmt.Sprintf(`(%s)`, a.Val)
		case astro.SpreadAttribute:
			return fmt.Sprintf(`...(%s)`, a.Key)
		case astro.ShorthandAttribute:
			return fmt.Sprintf(`(%s)`, strings.TrimSpace(a.Val))
		case astro.TemplateLiteralAttribute:
			return fmt.Sprintf("`%s`", a.Key)
		}
	}
	return ""
}
