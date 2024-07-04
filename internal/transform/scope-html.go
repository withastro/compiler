package transform

import (
	"fmt"
	"strings"

	astro "github.com/withastro/compiler/internal"
	"golang.org/x/net/html/atom"
)

func ScopeElement(n *astro.Node, opts TransformOptions) {
	if n.Type == astro.ElementNode {
		if _, noScope := NeverScopedElements[n.Data]; !noScope {
			injectScopedClass(n, opts)
		}
	}
}

func AddDefineVars(n *astro.Node, values []string) bool {
	if n.Type == astro.ElementNode && !n.Component {
		if _, noScope := NeverScopedElements[n.Data]; !noScope {
			if !IsImplicitNode(n) {
				injectDefineVars(n, values)
				return true
			}
		}
	}
	return false
}

func AnnotateElement(n *astro.Node, opts TransformOptions) {
	if n.Type == astro.ElementNode && !n.Component && !n.Fragment {
		if _, noScope := NeverScopedElements[n.Data]; !noScope {
			annotateElement(n, opts)
		}
	}
}

var NeverScopedElements map[string]bool = map[string]bool{
	"Fragment": true,
	"base":     true,
	"font":     true,
	"frame":    true,
	"frameset": true,
	"head":     true,
	"link":     true,
	"meta":     true,
	"noframes": true,
	"noscript": true,
	"script":   true,
	"style":    true,
	"slot":     true,
	"title":    true,
}

var NeverScopedSelectors map[string]bool = map[string]bool{
	":root": true,
}

func annotateElement(n *astro.Node, opts TransformOptions) {
	if n.DataAtom == atom.Html {
		return
	}
	n.Attr = append(n.Attr, astro.Attribute{
		Key:  "data-astro-source-file",
		Type: astro.QuotedAttribute,
		Val:  opts.Filename,
	})
}

func injectDefineVars(n *astro.Node, values []string) {
	definedVars := "$$definedVars"
	for i, attr := range n.Attr {
		if attr.Key == "style" {
			switch attr.Type {
			case astro.ShorthandAttribute:
				attr.Type = astro.ExpressionAttribute
				attr.Val = fmt.Sprintf("`${%s}; ${%s}`", attr.Key, definedVars)
				n.Attr[i] = attr
				return
			case astro.EmptyAttribute:
				attr.Type = astro.ExpressionAttribute
				attr.Val = definedVars
				n.Attr[i] = attr
				return
			case astro.QuotedAttribute:
				attr.Type = astro.ExpressionAttribute
				attr.Val = fmt.Sprintf("`${\"%s\"}; ${%s}`", attr.Val, definedVars)
				n.Attr[i] = attr
				return
			case astro.TemplateLiteralAttribute:
				attr.Type = astro.ExpressionAttribute
				attr.Val = fmt.Sprintf("`${`%s`}; ${%s}`", attr.Val, definedVars)
				n.Attr[i] = attr
				return
			case astro.ExpressionAttribute:
				attr.Type = astro.ExpressionAttribute
				trimmed := strings.TrimSpace(attr.Val)
				if trimmed[0] == '{' {
					attr.Val = fmt.Sprintf("[%s,%s]", trimmed, definedVars)
				} else {
					attr.Val = fmt.Sprintf("`${%s}; ${%s}`", attr.Val, definedVars)
				}
				n.Attr[i] = attr
				return
			}
		}
	}
	n.Attr = append(n.Attr, astro.Attribute{
		Key:  "style",
		Type: astro.ExpressionAttribute,
		Val:  definedVars,
	})
}

func injectScopedClass(n *astro.Node, opts TransformOptions) {
	hasSpreadAttr := false
	if opts.ScopedStyleStrategy != "attribute" {
		scopedClass := fmt.Sprintf(`astro-%s`, opts.Scope)

		for i, attr := range n.Attr {
			if !hasSpreadAttr && attr.Type == astro.SpreadAttribute {
				// We only handle this special case on built-in elements
				hasSpreadAttr = !n.Component
			}

			// If we find an existing class attribute, append the scoped class
			if attr.Key == "class" || (n.Component && attr.Key == "className") {
				switch attr.Type {
				case astro.ShorthandAttribute:
					if n.Component {
						attr.Val = fmt.Sprintf(`%s + " %s"`, attr.Key, scopedClass)
						attr.Type = astro.ExpressionAttribute
						n.Attr[i] = attr
						return
					}
				case astro.EmptyAttribute:
					// instead of an empty string
					attr.Type = astro.QuotedAttribute
					attr.Val = scopedClass
					n.Attr[i] = attr
					return
				case astro.QuotedAttribute, astro.TemplateLiteralAttribute:
					// as a plain string
					attr.Val = fmt.Sprintf(`%s %s`, attr.Val, scopedClass)
					n.Attr[i] = attr
					return
				case astro.ExpressionAttribute:
					// as an expression
					attr.Val = fmt.Sprintf(`((%s) ?? "") + " %s"`, attr.Val, scopedClass)
					n.Attr[i] = attr
					return
				}
			}

			if attr.Key == "class:list" {
				switch attr.Type {
				case astro.EmptyAttribute:
					// instead of an empty string
					attr.Type = astro.QuotedAttribute
					attr.Val = "astro-" + opts.Scope
					n.Attr[i] = attr
					return
				case astro.QuotedAttribute, astro.TemplateLiteralAttribute:
					// as a plain string
					attr.Val = attr.Val + " astro-" + opts.Scope
					n.Attr[i] = attr
					return
				case astro.ExpressionAttribute:
					// as an expression
					attr.Val = fmt.Sprintf(`[(%s), "%s"]`, attr.Val, scopedClass)
					n.Attr[i] = attr
					return
				}
			}
		}

		// If there's a spread attribute, `class` might be there, so do not inject `class` here
		// `class` will be injected by the `spreadAttributes` helper
		if hasSpreadAttr {
			return
		}
		// If we didn't find an existing class attribute, let's add one
		n.Attr = append(n.Attr, astro.Attribute{
			Key:  "class",
			Type: astro.QuotedAttribute,
			Val:  scopedClass,
		})
	} else {
		n.Attr = append(n.Attr, astro.Attribute{
			Key:  fmt.Sprintf(`data-astro-cid-%s`, opts.Scope),
			Type: astro.EmptyAttribute,
		})
	}

}
