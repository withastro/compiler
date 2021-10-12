// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package printer

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	. "github.com/snowpackjs/astro/internal"
	"github.com/snowpackjs/astro/internal/js_scanner"
	"github.com/snowpackjs/astro/internal/loc"
	"github.com/snowpackjs/astro/internal/sourcemap"
	"github.com/snowpackjs/astro/internal/transform"
	"golang.org/x/net/html/atom"
)

// Render renders the parse tree n to the given writer.
//
// Rendering is done on a 'best effort' basis: calling Parse on the output of
// Render will always result in something similar to the original tree, but it
// is not necessarily an exact clone unless the original tree was 'well-formed'.
// 'Well-formed' is not easily specified; the HTML5 specification is
// complicated.
//
// Calling Parse on arbitrary input typically results in a 'well-formed' parse
// tree. However, it is possible for Parse to yield a 'badly-formed' parse tree.
// For example, in a 'well-formed' parse tree, no <a> element is a child of
// another <a> element: parsing "<a><a>" results in two sibling elements.
// Similarly, in a 'well-formed' parse tree, no <a> element is a child of a
// <table> element: parsing "<p><table><a>" results in a <p> with two sibling
// children; the <a> is reparented to the <table>'s parent. However, calling
// Parse on "<a><table><a>" does not return an error, but the result has an <a>
// element with an <a> child, and is therefore not 'well-formed'.
//
// Programmatically constructed trees are typically also 'well-formed', but it
// is possible to construct a tree that looks innocuous but, when rendered and
// re-parsed, results in a different tree. A simple example is that a solitary
// text node would become a tree containing <html>, <head> and <body> elements.
// Another example is that the programmatic equivalent of "a<head>b</head>c"
// becomes "<html><head><head/><body>abc</body></html>".
func PrintToJS(sourcetext string, n *Node, opts transform.TransformOptions) PrintResult {
	p := &printer{
		opts:    opts,
		builder: sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(sourcetext, len(strings.Split(sourcetext, "\n")))),
	}
	return printToJs(p, n)
}

func PrintToJSFragment(sourcetext string, n *Node, opts transform.TransformOptions) PrintResult {
	p := &printer{
		opts:    opts,
		builder: sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(sourcetext, len(strings.Split(sourcetext, "\n")))),
	}
	return printToJs(p, n)
}

type RenderOptions struct {
	isRoot       bool
	isExpression bool
	depth        int
}

type ExtractedStatement struct {
	Content string
	Loc     loc.Loc
}

func printToJs(p *printer, n *Node) PrintResult {
	render1(p, n, RenderOptions{
		isRoot:       true,
		isExpression: false,
		depth:        0,
	})

	return PrintResult{
		Output:         p.output,
		SourceMapChunk: p.builder.GenerateChunk(p.output),
	}
}

func render1(p *printer, n *Node, opts RenderOptions) {
	depth := opts.depth

	// Root of the document, print all children
	if n.Type == DocumentNode {
		p.printInternalImports(p.opts.InternalURL)

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			render1(p, c, RenderOptions{
				isRoot:       true,
				isExpression: false,
				depth:        depth + 1,
			})
		}
		return
	}
	// Render frontmatter (will be the first node, if it exists)
	if n.Type == FrontmatterNode {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == TextNode {
				p.printInternalImports(p.opts.InternalURL)

				// This scanner returns a position where we should slice the frontmatter.
				// If it encounters any `await`ed code or code that accesses the `Astro` global,
				// `renderBodyStart` will be the index where we should split the frontmatter.
				// If we don't encounter any of those, `renderBodyStart` will be `-1`
				renderBodyStart := js_scanner.FindRenderBody([]byte(c.Data))
				p.addSourceMapping(n.Loc[0])
				if renderBodyStart == -1 {
					p.addSourceMapping(c.Loc[0])
					if js_scanner.AccessesPrivateVars([]byte(c.Data)) {
						panic(errors.New("Variables prefixed by \"$$\" are reserved for Astro's internal usage!"))
					}
					p.print(strings.Trim(c.Data, " \t\r\n"))

					printComponentImports(p, n.Parent, []byte(c.Data))

					// TODO: use the proper component name
					p.printFuncPrelude("$$Component")
				} else {
					importStatements := c.Data[0:renderBodyStart]
					renderBody := c.Data[renderBodyStart:]

					if js_scanner.HasExports([]byte(renderBody)) {
						panic(errors.New("Export statements must be placed at the top of .astro files!"))
					}
					// fmt.Println(js_scanner.AccessesPrivateVars([]byte(renderBody)))
					//  {
					// 	panic(errors.New("Variables prefixed by \"$$\" are reserved for Astro's internal usage!"))
					// }
					p.addSourceMapping(c.Loc[0])
					p.println(strings.Trim(importStatements, " \t\r\n"))

					printComponentImports(p, n.Parent, []byte(importStatements))

					// TODO: use the proper component name
					p.printFuncPrelude("$$Component")
					p.addSourceMapping(loc.Loc{Start: c.Loc[0].Start + renderBodyStart})
					p.print(renderBody)
				}

				if len(n.Parent.Styles) > 0 {
					p.println("const STYLES = [")
					for _, style := range n.Parent.Styles {
						p.printStyleOrScript(style)
					}
					p.println("];")
					p.addNilSourceMapping()
					p.println(fmt.Sprintf("%s.styles.add(...STYLES)", RESULT))
				}

				if len(n.Parent.Scripts) > 0 {
					p.println("const SCRIPTS = [")
					for _, script := range n.Parent.Scripts {
						p.printStyleOrScript(script)
					}
					p.println("];")
					p.addNilSourceMapping()
					p.println(fmt.Sprintf("%s.scripts.add(...SCRIPTS)", RESULT))
				}

				p.printReturnOpen()
			} else {
				render1(p, c, RenderOptions{
					isRoot:       false,
					isExpression: true,
					depth:        depth + 1,
				})
				p.addSourceMapping(loc.Loc{Start: n.Loc[1].Start - 3})
			}
		}
		return
	} else if !p.hasFuncPrelude {
		// Render func prelude. Will only run for the first non-frontmatter node
		// TODO: use the proper component name
		p.printFuncPrelude("$$Component")

		// If we haven't printed the funcPrelude but we do have Styles/Scripts, we need to print them!
		if len(n.Styles) > 0 {
			p.println("const STYLES = [")
			for _, style := range n.Styles {
				p.printStyleOrScript(style)
			}
			p.println("];")
			p.addNilSourceMapping()
			p.println(fmt.Sprintf("%s.styles.add(...STYLES)", RESULT))
		}
		if len(n.Scripts) > 0 {
			p.println("const SCRIPTS = [")
			for _, script := range n.Scripts {
				p.printStyleOrScript(script)
			}
			p.println("];")
			p.addNilSourceMapping()
			p.println(fmt.Sprintf("%s.scripts.add(...SCRIPTS)", RESULT))
		}

		p.printReturnOpen()
	}
	switch n.Type {
	case TextNode:
		if strings.TrimSpace(n.Data) == "" {
			p.addSourceMapping(n.Loc[0])
			p.print(n.Data)
			return
		}
		text := escapeText(n.Data)
		p.addSourceMapping(n.Loc[0])
		p.print(text)
		return
	case ElementNode:
		// No-op.
	case CommentNode:
		p.addSourceMapping(n.Loc[0])
		p.print("<!--")
		p.print(escapeText(n.Data))
		p.print("-->")
		return
	case DoctypeNode:
		p.print("<!DOCTYPE ")
		p.print(n.Data)
		if n.Attr != nil {
			var public, system string
			for _, a := range n.Attr {
				switch a.Key {
				case "public":
					public = a.Val
				case "system":
					system = a.Val
				}
			}
			if public != "" {
				p.print(" PUBLIC ")
				p.print(fmt.Sprintf(`"%s"`, public))
				if system != "" {
					p.print(" ")
					p.print(fmt.Sprintf(`"%s"`, system))
				}
			} else if system != "" {
				p.print(" SYSTEM ")
				p.print(fmt.Sprintf(`"%s"`, system))
			}
		}
		p.print(">")
		return
	case RawNode:
		p.print(n.Data)
		return
	}

	// Tip! Comment this block out to debug expressions
	if n.Expression {
		if n.FirstChild != nil {
			p.print("${")
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			p.addSourceMapping(c.Loc[0])
			if c.Type == TextNode {
				p.print(c.Data)
			} else {
				if c.PrevSibling == nil || (c.PrevSibling != nil && c.PrevSibling.Type == TextNode) {
					// TODO: where is this used?
					// c.NextSibling.Type != TextNode
					p.printTemplateLiteralOpen()
				}
				render1(p, c, RenderOptions{
					isRoot:       false,
					isExpression: true,
					depth:        depth + 1,
				})
				if c.NextSibling == nil || (c.NextSibling != nil && c.NextSibling.Type == TextNode) {
					p.printTemplateLiteralClose()
				}
			}
		}
		p.addSourceMapping(n.Loc[1])
		p.print("}")
		return
	}

	isComponent := (n.Component || n.CustomElement) && n.Data != "Fragment"
	isSlot := n.DataAtom == atom.Slot

	p.addSourceMapping(n.Loc[0])
	if isComponent {
		p.print(fmt.Sprintf("${%s(%s,'%s',", RENDER_COMPONENT, RESULT, n.Data))
	} else if isSlot {
		p.print(fmt.Sprintf("${%s(%s,%s[", RENDER_SLOT, RESULT, SLOTS))
	} else {
		p.print("<")
	}

	p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start + 1})

	// fmt.Println("OPEN", n.Data)

	if n.Fragment {
		p.print("Fragment")
	} else if !isSlot {
		if n.CustomElement {
			p.print(fmt.Sprintf("'%s'", n.Data))
		} else {
			p.print(n.Data)
		}
	}

	p.addSourceMapping(n.Loc[0])
	if isComponent {
		p.print(",")
		p.printAttributesToObject(n)
	} else if isSlot {
		if len(n.Attr) == 0 {
			p.print(`"default"`)
		} else {
			slotted := false
			for _, a := range n.Attr {
				if a.Key != "name" {
					continue
				}
				switch a.Type {
				case QuotedAttribute:
					p.addSourceMapping(a.ValLoc)
					p.print(`"` + a.Val + `"`)
					slotted = true
				default:
					panic("slot[name] must be a static string")
				}
				// if i != len(n.Attr)-1 {
				// 	p.print("")
				// }
			}
			if !slotted {
				p.print(`"default"`)
			}
		}
		p.print(`]`)
	} else {
		for _, a := range n.Attr {
			if a.Key == "slot" {
				if !((n.Parent.Component || n.Parent.CustomElement) && n.Parent.Data != "Fragment") {
					panic(`Element with a slot='...' attribute must be a child of a component or a descendant of a custom element`)
				}
			} else {
				p.printAttribute(a)
				p.addSourceMapping(n.Loc[0])
			}
		}
		p.addSourceMapping(n.Loc[0])
		p.print(">")
	}

	if voidElements[n.Data] {
		if n.FirstChild != nil {
			// return fmt.Errorf("html: void element <%s> has child nodes", n.Data)
		}
		return
	}

	// Add initial newline where there is danger of a newline beging ignored.
	if c := n.FirstChild; c != nil && c.Type == TextNode && strings.HasPrefix(c.Data, "\n") {
		switch n.Data {
		case "pre", "listing", "textarea":
			p.print("\n")
		}
	}

	// Render any child nodes.
	switch n.Data {
	case "iframe", "noembed", "noframes", "noscript", "plaintext", "script", "style", "xmp":
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == TextNode {
				p.print(escapeText(c.Data))
			} else {
				render1(p, c, RenderOptions{
					isRoot: false,
					depth:  depth + 1,
				})
			}
		}
		// if n.Data == "plaintext" {
		// 	// Don't render anything else. <plaintext> must be the
		// 	// last element in the file, with no closing tag.
		// 	return
		// }
	default:
		isAllWhiteSpace := false
		if isComponent || isSlot {
			isAllWhiteSpace = true
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				isAllWhiteSpace = c.Type == TextNode && strings.TrimSpace(c.Data) == ""
				if !isAllWhiteSpace {
					break
				}
			}
		}

		if !isAllWhiteSpace {
			switch true {
			case isComponent:
				p.print(`,{`)
				slottedChildren := make(map[string][]*Node)
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					slotProp := `"default"`
					for _, a := range c.Attr {
						if a.Key == "slot" {
							if a.Type == QuotedAttribute {
								slotProp = fmt.Sprintf(`"%s"`, a.Val)
							} else if a.Type == ExpressionAttribute {
								slotProp = fmt.Sprintf(`[%s]`, a.Val)
							} else {
								panic(`unknown slot attribute type`)
							}
						}
					}
					// Only slot ElementNodes or non-empty TextNodes!
					// CommentNode and others should not be slotted
					if c.Type == ElementNode || (c.Type == TextNode && strings.TrimSpace(c.Data) != "") {
						slottedChildren[slotProp] = append(slottedChildren[slotProp], c)
					}
				}
				// fix: sort keys for stable output
				slottedKeys := make([]string, 0, len(slottedChildren))
				for k := range slottedChildren {
					slottedKeys = append(slottedKeys, k)
				}
				sort.Strings(slottedKeys)
				for _, slotProp := range slottedKeys {
					children := slottedChildren[slotProp]
					p.print(fmt.Sprintf(`%s: () => `, slotProp))
					p.printTemplateLiteralOpen()
					for _, child := range children {
						render1(p, child, RenderOptions{
							isRoot:       false,
							isExpression: opts.isExpression,
							depth:        depth + 1,
						})
					}
					p.printTemplateLiteralClose()
					p.print(`,`)
				}
				p.print(`}`)
			case isSlot:
				p.print(`,`)
				p.printTemplateLiteralOpen()
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					render1(p, c, RenderOptions{
						isRoot:       false,
						isExpression: opts.isExpression,
						depth:        depth + 1,
					})
				}
				p.printTemplateLiteralClose()
			default:
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					render1(p, c, RenderOptions{
						isRoot:       false,
						isExpression: opts.isExpression,
						depth:        depth + 1,
					})
				}
			}
		}
	}

	if len(n.Loc) == 2 {
		p.addSourceMapping(n.Loc[1])
	} else {
		p.addSourceMapping(n.Loc[0])
	}
	if isComponent || isSlot {
		p.print(")}")
	} else {
		p.print(`</` + n.Data + `>`)
	}

	if opts.isRoot {
		p.printReturnClose()
		// TODO: use proper component name
		p.printFuncSuffix("$$Component")
	}
}

func printComponentImports(p *printer, doc *Node, source []byte) {
	// Only print this for components with hydrated components
	if len(doc.HydratedComponents) == 0 {
		return
	}

	var specs []string

	modCount := 1
	loc, specifier := js_scanner.NextImportSpecifier(source, 0)
	for loc != -1 {
		p.print(fmt.Sprintf("\nimport * as $$module%v from '%s';", modCount, specifier))
		specs = append(specs, specifier)
		loc, specifier = js_scanner.NextImportSpecifier(source, loc)
		modCount++
	}

	// Call createHydrationMap
	p.print(fmt.Sprintf("\nconst $$hydrationMap = %s('%s', [", CREATE_HYDRATION_MAP, p.opts.Filename))
	for i := 1; i < modCount; i++ {
		if i > 1 {
			p.print(", ")
		}
		p.print(fmt.Sprintf("{ module: $$module%v, specifier: '%s' }", i, specs[i-1]))
	}
	p.print(fmt.Sprintf("], ["))

	for i, node := range doc.HydratedComponents {
		if i > 0 {
			p.print(", ")
		}

		p.print(node.Data)
	}
	p.print("]);")
}

// Section 12.1.2, "Elements", gives this list of void elements. Void elements
// are those that can't have any contents.
//nolint
var voidElements = map[string]bool{
	"area":   true,
	"base":   true,
	"br":     true,
	"col":    true,
	"embed":  true,
	"hr":     true,
	"img":    true,
	"input":  true,
	"keygen": true, // "keygen" has been removed from the spec, but are kept here for backwards compatibility.
	"link":   true,
	"meta":   true,
	"param":  true,
	"source": true,
	"track":  true,
	"wbr":    true,
}
