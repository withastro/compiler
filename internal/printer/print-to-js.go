// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package printer

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	. "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/js_scanner"
	"github.com/withastro/compiler/internal/loc"
	"github.com/withastro/compiler/internal/sourcemap"
	"github.com/withastro/compiler/internal/transform"
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
func PrintToJS(sourcetext string, n *Node, cssLen int, opts transform.TransformOptions) PrintResult {
	p := &printer{
		opts:    opts,
		builder: sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(sourcetext, len(strings.Split(sourcetext, "\n")))),
	}
	return printToJs(p, n, cssLen, opts)
}

func PrintToJSFragment(sourcetext string, n *Node, cssLen int, opts transform.TransformOptions) PrintResult {
	p := &printer{
		opts:    opts,
		builder: sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(sourcetext, len(strings.Split(sourcetext, "\n")))),
	}
	return printToJs(p, n, cssLen, opts)
}

type RenderOptions struct {
	isRoot       bool
	isExpression bool
	depth        int
	cssLen       int
	opts         transform.TransformOptions
}

type ExtractedStatement struct {
	Content string
	Loc     loc.Loc
}

func printToJs(p *printer, n *Node, cssLen int, opts transform.TransformOptions) PrintResult {
	render1(p, n, RenderOptions{
		cssLen:       cssLen,
		isRoot:       true,
		isExpression: false,
		depth:        0,
		opts:         opts,
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
		if opts.opts.StaticExtraction {
			p.printCSSImports(opts.cssLen)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			render1(p, c, RenderOptions{
				isRoot:       false,
				isExpression: false,
				depth:        depth + 1,
				opts:         opts.opts,
			})
		}

		p.printReturnClose()
		p.printFuncSuffix(opts.opts)
		return
	}

	// Render frontmatter (will be the first node, if it exists)
	if n.Type == FrontmatterNode {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == TextNode {
				p.printInternalImports(p.opts.InternalURL)
				if opts.opts.StaticExtraction {
					p.printCSSImports(opts.cssLen)
				}

				// This scanner returns a position where we should slice the frontmatter.
				// If it encounters any `await`ed code or code that accesses the `Astro` global,
				// `renderBodyStart` will be the index where we should split the frontmatter.
				// If we don't encounter any of those, `renderBodyStart` will be `-1`
				renderBodyStart := js_scanner.FindRenderBody([]byte(c.Data))
				if len(n.Loc) > 0 {
					p.addSourceMapping(n.Loc[0])
				}
				if renderBodyStart == -1 {
					if len(c.Loc) > 0 {
						p.addSourceMapping(c.Loc[0])
					}
					preprocessed := js_scanner.HoistExports([]byte(c.Data))

					// 1. After imports put in the top-level Astro.
					p.printTopLevelAstro(opts.opts)

					if len(preprocessed.Hoisted) > 0 {
						for _, hoisted := range preprocessed.Hoisted {
							p.println(strings.TrimSpace(string(hoisted)))
						}
					}

					// 2. The frontmatter.
					p.print(strings.TrimSpace(c.Data))

					// 3. The metadata object
					p.printComponentMetadata(n.Parent, opts.opts, []byte(c.Data))

					p.printFuncPrelude(opts.opts)
				} else {
					importStatements := c.Data[0:renderBodyStart]
					content := c.Data[renderBodyStart:]
					preprocessed := js_scanner.HoistExports([]byte(content))
					renderBody := preprocessed.Body

					if js_scanner.HasExports(renderBody) {
						panic(errors.New("Export statements must be placed at the top of .astro files!"))
					}
					if len(c.Loc) > 0 {
						p.addSourceMapping(c.Loc[0])
					}
					p.println(strings.TrimSpace(importStatements))

					// 1. Component imports, if any exist.
					p.printComponentMetadata(n.Parent, opts.opts, []byte(importStatements))
					// 2. Top-level Astro global.
					p.printTopLevelAstro(opts.opts)

					if len(preprocessed.Hoisted) > 0 {
						for _, hoisted := range preprocessed.Hoisted {
							p.println(strings.TrimSpace(string(hoisted)))
						}
					}

					p.printFuncPrelude(opts.opts)
					if len(c.Loc) > 0 {
						p.addSourceMapping(loc.Loc{Start: c.Loc[0].Start + renderBodyStart})
					}
					p.print(strings.TrimSpace(string(preprocessed.Body)))
				}

				// Print empty just to ensure a newline
				p.println("")
				if len(n.Parent.Styles) > 0 {
					p.println("const STYLES = [")
					for _, style := range n.Parent.Styles {
						p.printStyleOrScript(opts, style)
					}
					p.println("];")
					p.addNilSourceMapping()
					p.println(fmt.Sprintf("for (const STYLE of STYLES) %s.styles.add(STYLE);", RESULT))
				}

				if !opts.opts.StaticExtraction && len(n.Parent.Scripts) > 0 {
					p.println("const SCRIPTS = [")
					for _, script := range n.Parent.Scripts {
						p.printStyleOrScript(opts, script)
					}
					p.println("];")
					p.addNilSourceMapping()
					p.println(fmt.Sprintf("for (const SCRIPT of SCRIPTS) %s.scripts.add(SCRIPT);", RESULT))
				}

				p.printReturnOpen()
			} else {
				render1(p, c, RenderOptions{
					isRoot:       false,
					isExpression: true,
					depth:        depth + 1,
					opts:         opts.opts,
				})
				if len(n.Loc) > 1 {
					p.addSourceMapping(loc.Loc{Start: n.Loc[1].Start - 3})
				}
			}
		}
		return
	} else if !p.hasFuncPrelude {
		p.printComponentMetadata(n.Parent, opts.opts, []byte{})
		p.printTopLevelAstro(opts.opts)

		// Render func prelude. Will only run for the first non-frontmatter node
		p.printFuncPrelude(opts.opts)
		// This just ensures a newline
		p.println("")

		// If we haven't printed the funcPrelude but we do have Styles/Scripts, we need to print them!
		if len(n.Parent.Styles) > 0 {
			p.println("const STYLES = [")
			for _, style := range n.Parent.Styles {
				p.printStyleOrScript(opts, style)
			}
			p.println("];")
			p.addNilSourceMapping()
			p.println(fmt.Sprintf("for (const STYLE of STYLES) %s.styles.add(STYLE);", RESULT))
		}
		if !opts.opts.StaticExtraction && len(n.Parent.Scripts) > 0 {
			p.println("const SCRIPTS = [")
			for _, script := range n.Parent.Scripts {
				p.printStyleOrScript(opts, script)
			}
			p.println("];")
			p.addNilSourceMapping()
			p.println(fmt.Sprintf("for (const SCRIPT of SCRIPTS) %s.scripts.add(SCRIPT);", RESULT))
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
		} else {
			p.print("${(void 0)")
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			p.addSourceMapping(c.Loc[0])
			if c.Type == TextNode {
				p.print(c.Data)
				continue
			}
			if c.PrevSibling == nil || c.PrevSibling.Type == TextNode {
				p.printTemplateLiteralOpen()
			}
			render1(p, c, RenderOptions{
				isRoot:       false,
				isExpression: true,
				depth:        depth + 1,
				opts:         opts.opts,
			})
			if c.NextSibling == nil || c.NextSibling.Type == TextNode {
				p.printTemplateLiteralClose()
			}
		}
		if len(n.Loc) >= 2 {
			p.addSourceMapping(n.Loc[1])
		}
		p.print("}")
		return
	}

	isFragment := n.Fragment
	isComponent := isFragment || n.Component || n.CustomElement
	isClientOnly := isComponent && transform.HasAttr(n, "client:only")
	isSlot := n.DataAtom == atom.Slot
	isImplicit := false
	for _, a := range n.Attr {
		if transform.IsImplictNodeMarker(a) {
			isImplicit = true
			break
		}
	}

	p.addSourceMapping(n.Loc[0])
	switch true {
	case isFragment:
		p.print(fmt.Sprintf("${%s(%s,'%s',", RENDER_COMPONENT, RESULT, "Fragment"))
	case isComponent:
		p.print(fmt.Sprintf("${%s(%s,'%s',", RENDER_COMPONENT, RESULT, n.Data))
	case isSlot:
		p.print(fmt.Sprintf("${%s(%s,%s[", RENDER_SLOT, RESULT, SLOTS))
	case isImplicit:
		// do nothing
	default:
		p.print("<")

	}

	p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start + 1})
	switch true {
	case isFragment:
		p.print("Fragment")
	case isClientOnly:
		p.print("null")
	case !isSlot && n.CustomElement:
		p.print(fmt.Sprintf("'%s'", n.Data))
	case !isSlot && !isImplicit:
		p.print(n.Data)
	}

	p.addSourceMapping(n.Loc[0])
	if isImplicit {
		// do nothing
	} else if isComponent {
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
			if transform.IsImplictNodeMarker(a) {
				continue
			}
			if a.Key == "slot" {
				if !(n.Parent.Component || n.Parent.CustomElement) {
					panic(`Element with a slot='...' attribute must be a child of a component or a descendant of a custom element`)
				}
				if n.Parent.CustomElement {
					p.printAttribute(a)
					p.addSourceMapping(n.Loc[0])
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

	if n.DataAtom == atom.Script || n.DataAtom == atom.Style {
		p.printDefineVars(n)
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
					opts:   opts.opts,
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
			case n.CustomElement:
				p.print(`,{`)
				p.print(fmt.Sprintf(`"%s": () => `, "default"))
				p.printTemplateLiteralOpen()
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					render1(p, c, RenderOptions{
						isRoot:       false,
						isExpression: opts.isExpression,
						depth:        depth + 1,
						opts:         opts.opts,
					})
				}
				p.printTemplateLiteralClose()
				p.print(`,}`)
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
							opts:         opts.opts,
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
						opts:         opts.opts,
					})
				}
				p.printTemplateLiteralClose()
			default:
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					render1(p, c, RenderOptions{
						isRoot:       false,
						isExpression: opts.isExpression,
						depth:        depth + 1,
						opts:         opts.opts,
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
	} else if !isImplicit {
		if n.DataAtom == atom.Head {
			p.printRenderHead()
		}
		p.print(`</` + n.Data + `>`)
	}
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
