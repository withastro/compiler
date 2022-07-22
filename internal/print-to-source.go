package astro

import (
	"fmt"
	"strings"
)

func PrintToSource(buf *strings.Builder, node *Node) {
	switch node.Type {
	case DocumentNode:
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			PrintToSource(buf, c)
		}
	case FrontmatterNode:
		buf.WriteString("---")
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			PrintToSource(buf, c)
		}
		buf.WriteString("---")
	case TextNode:
		buf.WriteString(node.Data)
	case ElementNode:
		isImplicit := false
		for _, a := range node.Attr {
			if a.Key == ImplicitNodeMarker {
				isImplicit = true
				break
			}
		}
		if !isImplicit {
			if node.Expression {
				buf.WriteString("{")
			} else {
				buf.WriteString(fmt.Sprintf(`<%s`, node.Data))
			}
			for _, attr := range node.Attr {
				if attr.Key == ImplicitNodeMarker {
					continue
				}
				if attr.Namespace != "" {
					buf.WriteString(attr.Namespace)
					buf.WriteString(":")
				}
				buf.WriteString(" ")
				switch attr.Type {
				case QuotedAttribute:
					buf.WriteString(attr.Key)
					buf.WriteString("=")
					buf.WriteString(`"` + attr.Val + `"`)
				case EmptyAttribute:
					buf.WriteString(attr.Key)
				case ExpressionAttribute:
					buf.WriteString(attr.Key)
					buf.WriteString("=")
					buf.WriteString(`{` + strings.TrimSpace(attr.Val) + `}`)
				case SpreadAttribute:
					buf.WriteString(`{...` + strings.TrimSpace(attr.Val) + `}`)
				case ShorthandAttribute:
					buf.WriteString(attr.Key)
					buf.WriteString("=")
					buf.WriteString(`{` + strings.TrimSpace(attr.Key) + `}`)
				case TemplateLiteralAttribute:
					buf.WriteString(attr.Key)
					buf.WriteString("=`" + strings.TrimSpace(attr.Val) + "`")
				}
			}
			if !node.Expression {
				buf.WriteString(`>`)
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			PrintToSource(buf, c)
		}
		if !isImplicit {
			if node.Expression {
				buf.WriteString("}")
			} else {
				buf.WriteString(fmt.Sprintf(`</%s>`, node.Data))
			}
		}
	}
}
