package transform

import (
	"fmt"
	"strings"

	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/loc"
	"golang.org/x/net/html/atom"
	a "golang.org/x/net/html/atom"
)

type TransformOptions struct {
	As               string
	Scope            string
	Filename         string
	Pathname         string
	InternalURL      string
	SourceMap        string
	Site             string
	ProjectRoot      string
	PreprocessStyle  interface{}
	StaticExtraction bool
}

func Transform(doc *astro.Node, opts TransformOptions) *astro.Node {
	shouldScope := len(doc.Styles) > 0 && ScopeStyle(doc.Styles, opts)
	walk(doc, func(n *astro.Node) {
		ExtractScript(doc, n, &opts)
		AddComponentProps(doc, n)
		if shouldScope {
			ScopeElement(n, opts)
		}
	})
	NormalizeSetDirectives(doc)

	// Important! Remove scripts from original location *after* walking the doc
	for _, script := range doc.Scripts {
		script.Parent.RemoveChild(script)
	}

	// Sometimes files have leading <script hoist> or <style>...
	// Since we can't detect a "component-only" file until after `parse`, we need to handle
	// them here. The component will be hoisted to the root of the document, `html` and `head` will be removed.
	if opts.As != "fragment" {
		var onlyComponent *astro.Node
		var rootNode *astro.Node
		walk(doc, func(n *astro.Node) {
			if p := n.Parent; n.Component && p != nil && (p.DataAtom == a.Head || p.DataAtom == a.Body) {
				if !hasSiblings(n) {
					onlyComponent = n
				}
				return
			}
			if n.DataAtom == a.Html && (!IsImplictNode(n) || childCount(n) == 1) {
				rootNode = n
				return
			}
		})

		if rootNode == nil {
			rootNode = doc
		}

		if onlyComponent != nil {
			p := onlyComponent.Parent
			if IsImplictNode(p) {
				onlyComponent.Parent.RemoveChild(onlyComponent)
				rootNode.AppendChild(onlyComponent)
				rootNode.RemoveChild(onlyComponent.PrevSibling)
				if rootNode.FirstChild != nil && IsImplictNode(rootNode.FirstChild) {
					rootNode.RemoveChild(rootNode.FirstChild)
				}
			}
		}
	}

	// If we've emptied out all the nodes, this was a Fragment that only contained hoisted elements
	// Add an empty FrontmatterNode to allow the empty component to be printed
	if doc.FirstChild == nil {
		empty := &astro.Node{
			Type: astro.FrontmatterNode,
		}
		empty.AppendChild(&astro.Node{
			Type: astro.TextNode,
			Data: "",
		})
		doc.AppendChild(empty)
	}

	return doc
}

func ExtractStyles(doc *astro.Node) {
	walk(doc, func(n *astro.Node) {
		if n.Type == astro.ElementNode && n.DataAtom == a.Style {
			if HasSetDirective(n) {
				return
			}
			// Do not extract <style> inside of SVGs
			if n.Parent != nil && n.Parent.DataAtom == atom.Svg {
				return
			}
			// prepend node to maintain authored order
			doc.Styles = append([]*astro.Node{n}, doc.Styles...)
		}
	})
	// Important! Remove styles from original location *after* walking the doc
	for _, style := range doc.Styles {
		style.Parent.RemoveChild(style)
	}
}

func NormalizeSetDirectives(doc *astro.Node) {
	var nodes []*astro.Node
	var directives []*astro.Attribute
	walk(doc, func(n *astro.Node) {
		if n.Type == astro.ElementNode && HasSetDirective(n) {
			for _, attr := range n.Attr {
				if attr.Key == "set:html" || attr.Key == "set:text" {
					nodes = append(nodes, n)
					directives = append(directives, &attr)
					return
				}
			}
		}
	})

	if len(nodes) > 0 {
		for i, n := range nodes {
			directive := directives[i]
			n.RemoveAttribute(directive.Key)
			expr := &astro.Node{
				Type:       astro.ElementNode,
				Data:       "astro:expression",
				Expression: true,
			}
			loc := make([]loc.Loc, 1)
			loc = append(loc, directive.ValLoc)
			data := directive.Val
			if directive.Key == "set:html" {
				data = fmt.Sprintf("$$unescapeHTML(%s)", data)
			}
			// TODO: remove this explicit escape call when it becomes the default
			if directive.Key == "set:text" {
				data = fmt.Sprintf("$$escapeHTML(%s)", data)
			}
			expr.AppendChild(&astro.Node{
				Type: astro.TextNode,
				Data: data,
				Loc:  loc,
			})

			shouldWarn := false
			// Remove all existing children
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if !shouldWarn {
					shouldWarn = c.Type == astro.CommentNode || (c.Type == astro.TextNode && len(strings.TrimSpace(c.Data)) != 0)
				}
				n.RemoveChild(c)
			}
			if shouldWarn {
				fmt.Printf("<%s> uses the \"%s\" directive, but has child nodes which will be overwritten. Remove the child nodes to suppress this warning.\n", n.Data, directive.Key)
			}
			n.AppendChild(expr)
		}
	}
}

// TODO: cleanup sibling whitespace after removing scripts/styles
// func removeSiblingWhitespace(n *astro.Node) {
// 	if c := n.NextSibling; c != nil && c.Type == astro.TextNode {
// 		content := strings.TrimSpace(c.Data)
// 		if len(content) == 0 {
// 			c.Parent.RemoveChild(c)
// 		}
// 	}
// }

func ExtractScript(doc *astro.Node, n *astro.Node, opts *TransformOptions) {
	if n.Type == astro.ElementNode && n.DataAtom == a.Script {
		if HasSetDirective(n) {
			return
		}
		// if <script hoist>, hoist to the document root
		// If also using define:vars, that overrides the hoist tag.
		if hasTruthyAttr(n, "hoist") && !HasAttr(n, "define:vars") {
			shouldAdd := true
			for _, attr := range n.Attr {
				if attr.Key == "src" {
					if attr.Type == astro.ExpressionAttribute {
						if opts.StaticExtraction {
							shouldAdd = false
							fmt.Printf("%s: <script hoist> uses the expression {%s} on the src attribute and will be ignore. Use a static string on the src attribute instead.\n", opts.Filename, attr.Val)
						}
						break
					}
				}
			}

			// prepend node to maintain authored order
			if shouldAdd {
				doc.Scripts = append([]*astro.Node{n}, doc.Scripts...)
			}
		}
	}
}

func AddComponentProps(doc *astro.Node, n *astro.Node) {
	if n.Type == astro.ElementNode && (n.Component || n.CustomElement) {
		for _, attr := range n.Attr {
			id := n.Data
			if n.CustomElement {
				id = fmt.Sprintf("'%s'", id)
			}

			if strings.HasPrefix(attr.Key, "client:") {
				parts := strings.Split(attr.Key, ":")
				directive := parts[1]

				// Add the hydration directive so it can be extracted statically.
				doc.HydrationDirectives[directive] = true

				hydrationAttr := astro.Attribute{
					Key: "client:component-hydration",
					Val: directive,
				}
				n.Attr = append(n.Attr, hydrationAttr)

				if attr.Key == "client:only" {
					doc.ClientOnlyComponents = append([]*astro.Node{n}, doc.ClientOnlyComponents...)
					break
				}
				// prepend node to maintain authored order
				doc.HydratedComponents = append([]*astro.Node{n}, doc.HydratedComponents...)
				pathAttr := astro.Attribute{
					Key:  "client:component-path",
					Val:  fmt.Sprintf("$$metadata.getPath(%s)", id),
					Type: astro.ExpressionAttribute,
				}
				n.Attr = append(n.Attr, pathAttr)

				exportAttr := astro.Attribute{
					Key:  "client:component-export",
					Val:  fmt.Sprintf("$$metadata.getExport(%s)", id),
					Type: astro.ExpressionAttribute,
				}
				n.Attr = append(n.Attr, exportAttr)
				break
			}
		}
	}
}

func walk(doc *astro.Node, cb func(*astro.Node)) {
	var f func(*astro.Node)
	f = func(n *astro.Node) {
		cb(n)
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
}

func hasSiblings(n *astro.Node) bool {
	if n.NextSibling == nil && n.PrevSibling == nil {
		return false
	}

	var flag bool
	if n.Parent != nil {
		for c := n.Parent.FirstChild; c != nil; c = c.NextSibling {
			if c == n {
				continue
			}
			if c.Type == astro.TextNode && strings.TrimSpace(c.Data) == "" {
				continue
			}
			if c.Type == astro.CommentNode {
				continue
			}
			flag = true
		}
	}

	return flag
}
