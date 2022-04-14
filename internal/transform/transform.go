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
	NormalizeFlowComponents(doc)

	// Important! Remove scripts from original location *after* walking the doc
	for _, script := range doc.Scripts {
		script.Parent.RemoveChild(script)
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
			if HasSetDirective(n) || HasInlineDirective(n) {
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

func createFragment(src *astro.Node) *astro.Node {
	fragment := &astro.Node{
		Type:      astro.ElementNode,
		Data:      "Fragment",
		Component: true,
		Fragment:  true,
		Loc:       make([]loc.Loc, 1),
	}
	for {
		child := src.FirstChild
		if child == nil {
			break
		}
		src.RemoveChild(child)
		fragment.AppendChild(child)
	}
	return fragment
}

type SwitchCase struct {
	name    string
	content *astro.Node
}

func NormalizeFlowComponents(doc *astro.Node) {
	var switches []*astro.Node
	var ifs []*astro.Node
	var fors []*astro.Node
	var withs []*astro.Node

	walk(doc, func(n *astro.Node) {
		if n.Type == astro.ElementNode {
			switch n.Data {
			case "Switch":
				switches = append(switches, n)
			case "If":
				ifs = append(ifs, n)
			case "For":
				fors = append(fors, n)
			case "With":
				withs = append(withs, n)
			}
		}
	})

	if len(switches) > 0 {
		for _, n := range switches {
			expr := &astro.Node{
				Type:       astro.ElementNode,
				Data:       "astro:expression",
				Expression: true,
			}
			loc := make([]loc.Loc, 1)
			cases := make([]SwitchCase, 0)
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type != astro.ElementNode {
					continue
				}
				if c.Data != "Case" && c.Data != "Default" {
					fmt.Printf("<%s> found as a child of <Switch>, but only <Case> and <Default> are supported!\n", c.Data)
					continue
				}
				content := createFragment(c)
				name := GetAttr(c, "is")
				if c.Data == "Default" {
					name = "default"
				}
				cases = append(cases, SwitchCase{
					name:    name,
					content: content,
				})
			}
			onAttr := GetAttr(n, "on")
			if onAttr == "" {
				onAttr = "true"
			}
			open := fmt.Sprintf(`() => { switch (%s) {`, onAttr)
			close := `}}`
			expr.AppendChild(&astro.Node{
				Type: astro.TextNode,
				Data: open,
				Loc:  loc,
			})
			for _, c := range cases {
				if c.name == "default" {
					expr.AppendChild(&astro.Node{
						Type: astro.TextNode,
						Data: "default: return (",
						Loc:  loc,
					})
				} else {
					expr.AppendChild(&astro.Node{
						Type: astro.TextNode,
						Data: fmt.Sprintf("case %s: return (", c.name),
						Loc:  loc,
					})
				}
				expr.AppendChild(c.content)
				expr.AppendChild(&astro.Node{
					Type: astro.TextNode,
					Data: ");",
					Loc:  loc,
				})
			}
			expr.AppendChild(&astro.Node{
				Type: astro.TextNode,
				Data: close,
				Loc:  loc,
			})
			n.Parent.InsertBefore(expr, n)
			n.Parent.RemoveChild(n)
		}
	}

	if len(ifs) > 0 {
		for _, n := range ifs {
			expr := &astro.Node{
				Type:       astro.ElementNode,
				Data:       "astro:expression",
				Expression: true,
			}
			loc := make([]loc.Loc, 1)
			cases := make([]SwitchCase, 0)
			toRemove := make([]*astro.Node, 0)
			for s := n; s != nil; s = s.NextSibling {
				if s.Type != astro.ElementNode {
					continue
				}
				if s.Data != "If" && s.Data != "Else" {
					break
				}
				content := createFragment(s)
				name := GetAttr(s, "is")
				if s.Data == "Else" {
					name = GetAttr(s, "if")
				}
				cases = append(cases, SwitchCase{
					name:    name,
					content: content,
				})
				if s.Data == "Else" {
					toRemove = append(toRemove, s)
				}
			}
			isAttr := GetAttr(n, "is")
			open := "() => {\n"
			close := `}`
			expr.AppendChild(&astro.Node{
				Type: astro.TextNode,
				Data: open,
				Loc:  loc,
			})
			for i, c := range cases {
				if i == 0 {
					expr.AppendChild(&astro.Node{
						Type: astro.TextNode,
						Data: fmt.Sprintf(`if (%s) return (`, isAttr),
						Loc:  loc,
					})
				} else if c.name == "" {
					expr.AppendChild(&astro.Node{
						Type: astro.TextNode,
						Data: "else return (",
						Loc:  loc,
					})
				} else {
					expr.AppendChild(&astro.Node{
						Type: astro.TextNode,
						Data: fmt.Sprintf("else if (%s) return (", c.name),
						Loc:  loc,
					})
				}
				expr.AppendChild(c.content)
				expr.AppendChild(&astro.Node{
					Type: astro.TextNode,
					Data: ")\n",
					Loc:  loc,
				})
			}
			expr.AppendChild(&astro.Node{
				Type: astro.TextNode,
				Data: close,
				Loc:  loc,
			})
			n.Parent.InsertBefore(expr, n)
			n.Parent.RemoveChild(n)
			for _, s := range toRemove {
				s.Parent.RemoveChild(s)
			}
		}
	}

	if len(fors) > 0 {
		for _, n := range fors {
			content := createFragment(n)
			expr := &astro.Node{
				Type:       astro.ElementNode,
				Data:       "astro:expression",
				Expression: true,
			}
			loc := make([]loc.Loc, 1)

			args := GetQuotedAttr(n, "let")
			ofAttr := GetAttr(n, "of")
			inAttr := GetAttr(n, "in")
			fromAttr := GetAttr(n, "from")
			toAttr := GetAttr(n, "to")
			repeatAttr := GetAttr(n, "repeat")
			stepAttr := GetAttr(n, "step")
			open := `() => { const $$res = [];`
			callParams := ""
			loop := ""
			if ofAttr != "" {
				loop = fmt.Sprintf("let $$index = -1; for (const $$value of %s) { $$index++; $$res.push(((%s) => { return ", ofAttr, args)
				callParams = "$$value, $$index"
			}
			if inAttr != "" {
				loop = fmt.Sprintf("for (const $$key in %s) { const $$value = %s[$$key]; $$res.push(((%s) => { return ", inAttr, inAttr, args)
				callParams = "$$key, $$value"
			}
			if toAttr != "" {
				if fromAttr == "" {
					fromAttr = "0"
				}
				if stepAttr == "" {
					stepAttr = "1"
				}
				loop = fmt.Sprintf("const $$from = %s; const $$to = %s; const $$step = %s; const $$dir = $$to > $$from ? 1 : -1; for (const $$index of Array.from({ length: Math.floor(Math.abs(($$to - $$from)) / $$step) + 1 }, (_, i) => $$from + (i * $$step * $$dir))) { $$res.push(((%s) => { return ", fromAttr, toAttr, stepAttr, args)
				callParams = "$$index"
			}
			if repeatAttr != "" {
				loop = fmt.Sprintf("for (const $$index of Array.from({ length: %s }, (_, i) => i)) { $$res.push(((%s) => { return ", repeatAttr, args)
				callParams = "$$index"
			}
			expr.AppendChild(&astro.Node{
				Type: astro.TextNode,
				Data: open,
				Loc:  loc,
			})
			expr.AppendChild(&astro.Node{
				Type: astro.TextNode,
				Data: loop,
				Loc:  loc,
			})
			expr.AppendChild(content)
			expr.AppendChild(&astro.Node{
				Type: astro.TextNode,
				Data: fmt.Sprintf(`}).call(null, %s))} return $$res; }`, callParams),
				Loc:  loc,
			})

			n.Parent.InsertBefore(expr, n)
			n.Parent.RemoveChild(n)
		}
	}

	if len(withs) > 0 {
		for _, n := range withs {
			keys := make([]string, 0)
			values := make([]string, 0)
			for _, a := range n.Attr {
				if a.Type == astro.SpreadAttribute {
					fmt.Printf("<With> does not support spread attributes! Please replace `{...%s}`\n", a.Key)
					continue
				}
				keys = append(keys, a.Key)
				values = append(values, GetAttr(n, a.Key))
			}
			content := createFragment(n)
			expr := &astro.Node{
				Type:       astro.ElementNode,
				Data:       "astro:expression",
				Expression: true,
			}
			loc := make([]loc.Loc, 1)
			open := fmt.Sprintf(`() => function(%s) { return (`, strings.Join(keys, ", "))
			expr.AppendChild(&astro.Node{
				Type: astro.TextNode,
				Data: open,
				Loc:  loc,
			})
			expr.AppendChild(content)
			expr.AppendChild(&astro.Node{
				Type: astro.TextNode,
				Data: fmt.Sprintf(`)}.call(null, %s)`, strings.Join(values, ", ")),
				Loc:  loc,
			})
			n.Parent.InsertBefore(expr, n)
			n.Parent.RemoveChild(n)
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
		if HasSetDirective(n) || HasInlineDirective(n) {
			return
		}

		// if <script>, hoist to the document root
		// If also using define:vars, that overrides the hoist tag.
		if (hasTruthyAttr(n, "hoist") && !HasAttr(n, "define:vars")) ||
			len(n.Attr) == 0 || (len(n.Attr) == 1 && n.Attr[0].Key == "src") {
			shouldAdd := true
			for _, attr := range n.Attr {
				if attr.Key == "hoist" {
					fmt.Printf("%s: <script hoist> is no longer needed. You may remove the `hoist` attribute.\n", opts.Filename)
				}
				if attr.Key == "src" {
					if attr.Type == astro.ExpressionAttribute {
						if opts.StaticExtraction {
							shouldAdd = false
							fmt.Printf("%s: <script> uses the expression {%s} on the src attribute and will be ignored. Use a string literal on the src attribute instead.\n", opts.Filename, attr.Val)
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
