package transform

import (
	"fmt"
	"strings"

	astro "github.com/snowpackjs/astro/internal"
	tycho "github.com/snowpackjs/astro/internal"
	"golang.org/x/net/html/atom"
	a "golang.org/x/net/html/atom"
)

type TransformOptions struct {
	As              string
	Scope           string
	Filename        string
	InternalURL     string
	SourceMap       string
	Site            string
	ProjectRoot     string
	PreprocessStyle interface{}
}

func Transform(doc *tycho.Node, opts TransformOptions) *tycho.Node {
	shouldScope := len(doc.Styles) > 0 && ScopeStyle(doc.Styles, opts)
	walk(doc, func(n *tycho.Node) {
		ExtractScript(doc, n)
		AddComponentProps(doc, n)
		if shouldScope {
			ScopeElement(n, opts)
		}
	})

	// Important! Remove scripts from original location *after* walking the doc
	for _, script := range doc.Scripts {
		script.Parent.RemoveChild(script)
	}

	// Sometimes files have leading <script hoist> or <style>...
	// Since we can't detect a "component-only" file until after `parse`, we need to handle
	// them here. The component will be hoisted to the root of the document, `html` and `head` will be removed.
	if opts.As != "Fragment" {
		var onlyComponent *tycho.Node
		var rootNode *tycho.Node
		walk(doc, func(n *tycho.Node) {
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

func ExtractStyles(doc *tycho.Node) {
	walk(doc, func(n *tycho.Node) {
		if n.Type == tycho.ElementNode && n.DataAtom == a.Style {
			// Do not extract <style> inside of SVGs
			if n.Parent != nil && n.Parent.DataAtom == atom.Svg {
				return
			}
			// prepend node to maintain authored order
			doc.Styles = append([]*tycho.Node{n}, doc.Styles...)
		}
	})
	// Important! Remove styles from original location *after* walking the doc
	for _, style := range doc.Styles {
		style.Parent.RemoveChild(style)
	}
}

// TODO: cleanup sibling whitespace after removing scripts/styles
// func removeSiblingWhitespace(n *tycho.Node) {
// 	if c := n.NextSibling; c != nil && c.Type == tycho.TextNode {
// 		content := strings.TrimSpace(c.Data)
// 		if len(content) == 0 {
// 			c.Parent.RemoveChild(c)
// 		}
// 	}
// }

func ExtractScript(doc *tycho.Node, n *tycho.Node) {
	if n.Type == tycho.ElementNode && n.DataAtom == a.Script {
		// if <script hoist>, hoist to the document root
		if hasTruthyAttr(n, "hoist") {
			// prepend node to maintain authored order
			doc.Scripts = append([]*tycho.Node{n}, doc.Scripts...)
		}
	}
}

func AddComponentProps(doc *tycho.Node, n *tycho.Node) {
	if n.Type == tycho.ElementNode && (n.Component || n.CustomElement) {
		for _, attr := range n.Attr {
			id := n.Data
			if n.CustomElement {
				id = fmt.Sprintf("'%s'", id)
			}

			if strings.HasPrefix(attr.Key, "client:") {
				if attr.Key == "client:only" {
					doc.ClientOnlyComponents = append([]*tycho.Node{n}, doc.ClientOnlyComponents...)
					break
				}
				// prepend node to maintain authored order
				doc.HydratedComponents = append([]*tycho.Node{n}, doc.HydratedComponents...)
				pathAttr := tycho.Attribute{
					Key:  "client:component-path",
					Val:  fmt.Sprintf("$$metadata.getPath(%s)", id),
					Type: tycho.ExpressionAttribute,
				}
				n.Attr = append(n.Attr, pathAttr)

				exportAttr := tycho.Attribute{
					Key:  "client:component-export",
					Val:  fmt.Sprintf("$$metadata.getExport(%s)", id),
					Type: tycho.ExpressionAttribute,
				}
				n.Attr = append(n.Attr, exportAttr)
				break
			}
		}
	}
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

func hasSiblings(n *tycho.Node) bool {
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
