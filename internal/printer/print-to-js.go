// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package printer

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode"

	. "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/handler"
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
func PrintToJS(sourcetext string, n *Node, cssLen int, opts transform.TransformOptions, h *handler.Handler) PrintResult {
	p := &printer{
		sourcetext: sourcetext,
		opts:       opts,
		builder:    sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(sourcetext, len(strings.Split(sourcetext, "\n")))),
		handler:    h,
	}
	return printToJs(p, n, cssLen, opts)
}

type RenderOptions struct {
	isRoot           bool
	isExpression     bool
	depth            int
	cssLen           int
	opts             transform.TransformOptions
	printedMaybeHead *bool
}

type ExtractedStatement struct {
	Content string
	Loc     loc.Loc
}

func printToJs(p *printer, n *Node, cssLen int, opts transform.TransformOptions) PrintResult {
	printedMaybeHead := false
	render1(p, n, RenderOptions{
		cssLen:           cssLen,
		isRoot:           true,
		isExpression:     false,
		depth:            0,
		opts:             opts,
		printedMaybeHead: &printedMaybeHead,
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
		p.addNilSourceMapping()
		p.printInternalImports(p.opts.InternalURL, &opts)
		if opts.opts.StaticExtraction && n.FirstChild != nil && n.FirstChild.Type != FrontmatterNode {
			p.printCSSImports(opts.cssLen)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			render1(p, c, RenderOptions{
				isRoot:           false,
				isExpression:     false,
				depth:            depth + 1,
				opts:             opts.opts,
				cssLen:           opts.cssLen,
				printedMaybeHead: opts.printedMaybeHead,
			})
		}

		p.printReturnClose()
		p.printFuncSuffix(opts.opts)
		return
	}

	// Render frontmatter (will be the first node, if it exists)
	if n.Type == FrontmatterNode {
		printFrontmatterNode(p, n, opts)
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
			definedVars := transform.GetDefineVars(n.Parent.Styles)
			if len(definedVars) > 0 {
				p.printf("const $$definedVars = %s([%s]);\n", DEFINE_STYLE_VARS, strings.Join(definedVars, ","))
			}
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
		printTextNode(p, n)
		return
	case ElementNode:
		// No-op.
	case CommentNode:
		printCommentNode(p, n)
		return
	case DoctypeNode:
		// Doctype doesn't get printed because the Astro runtime always appends it
		return
	case RawNode:
		p.print(n.Data)
		return
	case RenderHeadNode:
		p.printMaybeRenderHead()
		*opts.printedMaybeHead = true
		return
	}

	// Tip! Comment this block out to debug expressions
	if n.Expression {
		if n.FirstChild == nil {
			p.print("${(void 0)")
		} else if expressionOnlyHasCommentBlock(n) {
			// we do not print expressions that only contain comment blocks
			return
		} else {
			p.print("${")
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == TextNode {
				p.printTextWithSourcemap(c.Data, c.Loc[0])
				continue
			}
			if c.PrevSibling == nil || c.PrevSibling.Type == TextNode {
				p.printTemplateLiteralOpen()
			}
			render1(p, c, RenderOptions{
				isRoot:           false,
				isExpression:     true,
				depth:            depth + 1,
				opts:             opts.opts,
				cssLen:           opts.cssLen,
				printedMaybeHead: opts.printedMaybeHead,
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
		if isSlot && a.Key == "is:inline" {
			isSlot = false
		}
		if transform.IsImplictNodeMarker(a) {
			isImplicit = true
		}
	}

	switch true {
	case isFragment:
		p.addSourceMapping(n.Loc[0])
		p.print(fmt.Sprintf("${%s(%s,'%s',", RENDER_COMPONENT, RESULT, "Fragment"))
	case isComponent:
		p.addSourceMapping(n.Loc[0])
		p.print(fmt.Sprintf("${%s(%s,'%s',", RENDER_COMPONENT, RESULT, n.Data))
	case isSlot:
		p.addSourceMapping(n.Loc[0])
		p.print(fmt.Sprintf("${%s(%s,%s[", RENDER_SLOT, RESULT, SLOTS))
	case isImplicit:
		// do nothing
	default:
		// Before the first non-head element, inject $$maybeRender($$result)
		// This is for pages that do not contain an explicit head element
		switch n.DataAtom {
		case atom.Html, atom.Head, atom.Base, atom.Basefont, atom.Bgsound, atom.Link, atom.Meta, atom.Noframes, atom.Script, atom.Style, atom.Template, atom.Title:
			break
		default:
			if !*opts.printedMaybeHead {
				*opts.printedMaybeHead = true
				p.printMaybeRenderHead()
			}
		}
		p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start - 1})
		p.print("<")
	}

	switch true {
	case isFragment:
		p.print("Fragment")
	case isClientOnly:
		p.print("null")
	case !isSlot && n.CustomElement:
		p.print(fmt.Sprintf("'%s'", n.Data))
	case !isSlot && !isImplicit:
		// Print the tag name
		p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start})
		p.print(n.Data)
		p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start + len(n.Data)})
	}

	// p.addSourceMapping(n.Loc[0])
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
					p.print(`"` + escapeDoubleQuote(a.Val) + `"`)
					slotted = true
				default:
					p.handler.AppendError(&loc.ErrorWithRange{
						Code:  loc.ERROR_UNSUPPORTED_SLOT_ATTRIBUTE,
						Text:  "slot[name] must be a static string",
						Range: loc.Range{Loc: a.ValLoc, Len: len(a.Val)},
					})
				}
			}
			if !slotted {
				p.print(`"default"`)
			}
		}
		p.print(`]`)
	} else {
		for _, a := range n.Attr {
			if transform.IsImplictNodeMarker(a) || a.Key == "is:inline" {
				continue
			}
			if a.Key == "slot" {
				if n.Parent.Component || n.Parent.Expression {
					continue
				}
				// Note: if we encounter "slot" NOT inside a component, that's fine
				// These should be perserved in the output
				p.printAttribute(a, n)
			} else {
				p.printAttribute(a, n)
			}
		}
		if !voidElements[n.Data] {
			p.print(">")
		}
	}

	if voidElements[n.Data] {
		if len(n.Loc) > 1 {
			p.addSourceMapping(n.Loc[1])
		}
		p.print(">")
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
		p.printDefineVarsOpen(n)
	}

	// Render any child nodes.
	switch n.Data {
	case "iframe", "noembed", "noframes", "noscript", "plaintext", "script", "style", "xmp":
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == TextNode {
				p.print(escapeText(c.Data))
			} else {
				render1(p, c, RenderOptions{
					isRoot:           false,
					isExpression:     opts.isExpression,
					depth:            depth + 1,
					opts:             opts.opts,
					cssLen:           opts.cssLen,
					printedMaybeHead: opts.printedMaybeHead,
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
						isRoot:           false,
						isExpression:     opts.isExpression,
						depth:            depth + 1,
						opts:             opts.opts,
						cssLen:           opts.cssLen,
						printedMaybeHead: opts.printedMaybeHead,
					})
				}
				p.printTemplateLiteralClose()
				p.print(`,}`)
			case isComponent:
				p.print(`,`)
				slottedChildren := make(map[string][]*Node)
				conditionalSlottedChildren := make([][]*Node, 0)
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					slotProp := `"default"`
					for _, a := range c.Attr {
						if a.Key == "slot" {
							if a.Type == QuotedAttribute {
								slotProp = fmt.Sprintf(`"%s"`, escapeDoubleQuote(a.Val))
							} else if a.Type == ExpressionAttribute {
								slotProp = fmt.Sprintf(`[%s]`, a.Val)
							} else {
								p.handler.AppendError(&loc.ErrorWithRange{
									Code:  loc.ERROR_UNSUPPORTED_SLOT_ATTRIBUTE,
									Text:  "slot[name] must be a static string",
									Range: loc.Range{Loc: a.ValLoc, Len: len(a.Val)},
								})
							}
						}
					}
					if c.Expression {
						nestedSlots := make([]string, 0)
						for c1 := c.FirstChild; c1 != nil; c1 = c1.NextSibling {
							for _, a := range c1.Attr {
								if a.Key == "slot" {
									if a.Type == QuotedAttribute {
										nestedSlotProp := fmt.Sprintf(`"%s"`, escapeDoubleQuote(a.Val))
										nestedSlots = append(nestedSlots, nestedSlotProp)
									} else if a.Type == ExpressionAttribute {
										nestedSlotProp := fmt.Sprintf(`[%s]`, a.Val)
										nestedSlots = append(nestedSlots, nestedSlotProp)
									} else {
										panic(`unknown slot attribute type`)
									}
								}
							}
						}

						if len(nestedSlots) == 1 {
							slotProp = nestedSlots[0]
							slottedChildren[slotProp] = append(slottedChildren[slotProp], c)
							continue
						} else if len(nestedSlots) > 1 {
							conditionalChildren := make([]*Node, 0)
						child_loop:
							for c1 := c.FirstChild; c1 != nil; c1 = c1.NextSibling {
								for _, a := range c1.Attr {
									if a.Key == "slot" {
										if a.Type == QuotedAttribute {
											nestedSlotProp := fmt.Sprintf(`"%s"`, escapeDoubleQuote(a.Val))
											nestedSlots = append(nestedSlots, nestedSlotProp)
											conditionalChildren = append(conditionalChildren, &Node{Type: TextNode, Data: fmt.Sprintf("{%s: () => ", nestedSlotProp), Loc: make([]loc.Loc, 1)})
											conditionalChildren = append(conditionalChildren, c1)
											conditionalChildren = append(conditionalChildren, &Node{Type: TextNode, Data: "}", Loc: make([]loc.Loc, 1)})
											continue child_loop
										} else if a.Type == ExpressionAttribute {
											nestedSlotProp := fmt.Sprintf(`[%s]`, a.Val)
											nestedSlots = append(nestedSlots, nestedSlotProp)
											conditionalChildren = append(conditionalChildren, &Node{Type: TextNode, Data: fmt.Sprintf("{%s: () => ", nestedSlotProp), Loc: make([]loc.Loc, 1)})
											conditionalChildren = append(conditionalChildren, c1)
											conditionalChildren = append(conditionalChildren, &Node{Type: TextNode, Data: "}", Loc: make([]loc.Loc, 1)})
											continue child_loop
										} else {
											panic(`unknown slot attribute type`)
										}
									}
								}
								conditionalChildren = append(conditionalChildren, c1)
							}
							conditionalSlottedChildren = append(conditionalSlottedChildren, conditionalChildren)
							continue
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
				if len(conditionalSlottedChildren) > 0 {
					p.print(`$$mergeSlots(`)
				}
				p.print(`{`)
				if len(slottedKeys) > 0 {
					for _, slotProp := range slottedKeys {
						children := slottedChildren[slotProp]
						p.print(fmt.Sprintf(`%s: () => `, slotProp))
						p.printTemplateLiteralOpen()
						for _, child := range children {
							render1(p, child, RenderOptions{
								isRoot:           false,
								isExpression:     opts.isExpression,
								depth:            depth + 1,
								opts:             opts.opts,
								cssLen:           opts.cssLen,
								printedMaybeHead: opts.printedMaybeHead,
							})
						}
						p.printTemplateLiteralClose()
						p.print(`,`)
					}
				}
				p.print(`}`)
				if len(conditionalSlottedChildren) > 0 {
					for _, children := range conditionalSlottedChildren {
						p.print(",")
						for _, child := range children {
							if child.Type == ElementNode {
								p.printTemplateLiteralOpen()
							}
							render1(p, child, RenderOptions{
								isRoot:           false,
								isExpression:     opts.isExpression,
								depth:            depth + 1,
								opts:             opts.opts,
								cssLen:           opts.cssLen,
								printedMaybeHead: opts.printedMaybeHead,
							})
							if child.Type == ElementNode {
								p.printTemplateLiteralClose()
							}
						}
					}
					p.print(`)`)
				}
			case isSlot:
				p.print(`,`)
				p.printTemplateLiteralOpen()
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					render1(p, c, RenderOptions{
						isRoot:           false,
						isExpression:     opts.isExpression,
						depth:            depth + 1,
						opts:             opts.opts,
						cssLen:           opts.cssLen,
						printedMaybeHead: opts.printedMaybeHead,
					})
				}
				p.printTemplateLiteralClose()
			default:
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					render1(p, c, RenderOptions{
						isRoot:           false,
						isExpression:     opts.isExpression,
						depth:            depth + 1,
						opts:             opts.opts,
						cssLen:           opts.cssLen,
						printedMaybeHead: opts.printedMaybeHead,
					})
				}
			}
		}
	}

	if n.DataAtom == atom.Script || n.DataAtom == atom.Style {
		p.printDefineVarsClose(n)
	}
	if isComponent || isSlot {
		p.print(")}")
	} else if !isImplicit {
		if n.DataAtom == atom.Head {
			*opts.printedMaybeHead = true
			p.printRenderHead()
		}
		start := n.Loc[0].Start - 2
		if len(n.Loc) > 1 {
			start = n.Loc[1].Start - 2
		}
		p.addSourceMapping(loc.Loc{Start: start})
		p.print(`</`)
		start += 2
		p.addSourceMapping(loc.Loc{Start: start})
		p.print(n.Data)
		start += len(n.Data)
		p.addSourceMapping(loc.Loc{Start: start})
		p.print(`>`)
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

// Returns true if the expression only contains a comment block (e.g. {/* a comment */})
func expressionOnlyHasCommentBlock(n *Node) bool {
	clean, _ := removeComments(n.FirstChild.Data)
	return n.FirstChild.NextSibling == nil &&
		n.FirstChild.Type == TextNode &&
		// removeComments iterates over text and most of the time we won't be parsing comments so lets check if text starts with /* before iterating
		strings.HasPrefix(strings.TrimLeftFunc(n.FirstChild.Data, unicode.IsSpace), "/*") &&
		len(clean) == 0
}

func printFrontmatterNode(p *printer, n *Node, opts RenderOptions) {
	start := 0
	if len(n.Loc) > 0 {
		start = n.Loc[0].Start
	}
	p.addSourceMapping(loc.Loc{Start: start})
	p.println("// ---")
	if n.FirstChild == nil {
		p.printCSSImports(opts.cssLen)
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == TextNode {
			p.addNilSourceMapping()
			p.printInternalImports(p.opts.InternalURL, &opts)

			if len(c.Loc) > 0 {
				start = c.Loc[0].Start
			}

			statements := js_scanner.HoistStatements([]byte(c.Data), start)
			for _, statement := range statements.Imports {
				p.printNewlineIfNeeded()
				p.printTextWithSourcemap(string(bytes.TrimSpace(statement.Value)), loc.Loc{Start: statement.Start})
			}

			if opts.opts.StaticExtraction {
				p.addNilSourceMapping()
				p.printCSSImports(opts.cssLen)
			}

			// 1. Component imports, if any exist.
			// p.printComponentMetadata(n.Parent, opts.opts, []byte(importStatements))

			// 2. Top-level Astro global.
			p.printTopLevelAstro(opts.opts)

			// TODO: ensure exports are properly hoisted
			// for _, statement := range statements.Exports {
			// 	p.printNewlineIfNeeded()
			// 	p.printTextWithSourcemap(string(bytes.TrimSpace(statement.Value)), loc.Loc{Start: statement.Start})
			// }

			p.printFuncPrelude(opts.opts)

			for _, statement := range statements.Body {
				p.printTextWithSourcemap(string(statement.Value), loc.Loc{Start: statement.Start})
			}

			// Print empty just to ensure a newline
			p.printNewlineIfNeeded()

			if len(n.Parent.Styles) > 0 {
				definedVars := transform.GetDefineVars(n.Parent.Styles)
				if len(definedVars) > 0 {
					p.printf("const $$definedVars = %s([%s]);\n", DEFINE_STYLE_VARS, strings.Join(definedVars, ","))
				}
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
				isRoot:           false,
				isExpression:     true,
				depth:            opts.depth + 1,
				opts:             opts.opts,
				cssLen:           opts.cssLen,
				printedMaybeHead: opts.printedMaybeHead,
			})
			if len(n.Loc) > 1 {
				p.addSourceMapping(loc.Loc{Start: n.Loc[1].Start - 3})
			}
		}
	}
}

func printCommentNode(p *printer, n *Node) {
	p.printSegments([]string{"<!--", escapeText(n.Data), "-->"}, n.Loc[0].Start-4)
}

func printTextNode(p *printer, n *Node) {
	p.printTextWithSourcemap(escapeText(n.Data), n.Loc[0])
}
