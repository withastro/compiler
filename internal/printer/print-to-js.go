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
	"github.com/withastro/compiler/internal/helpers"
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
func PrintToJS(sourcetext string, doc *Node, s *js_scanner.Js_scanner, cssLen int, opts transform.TransformOptions, h *handler.Handler) PrintResult {
	p := &printer{
		sourcetext: sourcetext,
		opts:       opts,
		scanner:    s,
		builder:    sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(sourcetext, len(strings.Split(sourcetext, "\n")))),
		handler:    h,
	}
	return printToJs(p, doc, cssLen, opts)
}

type RenderOptions struct {
	isRoot           bool
	isExpression     bool
	depth            int
	cssLen           int
	opts             transform.TransformOptions
	printedMaybeHead *bool
	scriptCount      *int
}

type ExtractedStatement struct {
	Content string
	Loc     loc.Loc
}

func printToJs(p *printer, n *Node, cssLen int, opts transform.TransformOptions) PrintResult {
	printedMaybeHead := false
	scriptCount := 0
	render1(p, n, RenderOptions{
		cssLen:           cssLen,
		isRoot:           true,
		isExpression:     false,
		depth:            0,
		opts:             opts,
		printedMaybeHead: &printedMaybeHead,
		scriptCount:      &scriptCount,
	})

	return PrintResult{
		Output:         p.output,
		SourceMapChunk: p.builder.GenerateChunk(p.output),
	}
}

const whitespace = " \t\r\n\f"

// Returns true if the expression only contains a comment block (e.g. {/* a comment */})
func expressionOnlyHasComment(n *Node) bool {
	if n.FirstChild == nil {
		return false
	}
	clean := helpers.RemoveComments(n.FirstChild.Data)
	trimmedData := strings.TrimLeft(n.FirstChild.Data, whitespace)
	result := n.FirstChild.NextSibling == nil &&
		n.FirstChild.Type == TextNode &&
		// RemoveComments iterates over text and most of the time we won't be parsing comments so lets check if text starts with /* or // before iterating
		(strings.HasPrefix(trimmedData, "/*") || strings.HasPrefix(trimmedData, "//")) &&
		len(clean) == 0
	return result
}

func emptyTextNodeWithoutSiblings(n *Node) bool {
	if strings.TrimSpace(n.Data) != "" {
		return false
	}
	if n.PrevSibling == nil {
		return n.NextSibling == nil || n.NextSibling.Expression
	} else {
		return n.PrevSibling.Expression
	}
}

func render1(p *printer, n *Node, opts RenderOptions) {
	depth := opts.depth

	if n.Transition {
		p.needsTransitionCSS = true
	}

	// Root of the document, print all children
	if n.Type == DocumentNode {
		p.printInternalImports(p.opts.InternalURL, &opts)
		if n.FirstChild != nil && n.FirstChild.Type != FrontmatterNode {
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
				scriptCount:      opts.scriptCount,
			})
		}

		p.printReturnClose()
		p.printFuncSuffix(opts.opts, n)
		return
	}

	// Decide whether to print code for `Astro` global variable. Use a loose check for now.
	printAstroGlobal := strings.Contains(p.sourcetext, "Astro")

	// Render frontmatter (will be the first node, if it exists)
	if n.Type == FrontmatterNode {
		if n.FirstChild == nil {
			p.printCSSImports(opts.cssLen)
		} else if c := n.FirstChild; c.Type == TextNode {
			p.printInternalImports(p.opts.InternalURL, &opts)

			exportsPlusPlus := p.scanner.HoistExports()

			exports := exportsPlusPlus.Hoisted
			exportLocs := exportsPlusPlus.HoistedLocs

			bodies := exportsPlusPlus.Body
			bodiesLocs := exportsPlusPlus.BodyLocs

			start := 0
			if len(n.Loc) > 0 {
				start = c.Loc[0].Start
			}
			imports := p.scanner.HoistImports()
			for i, hoisted := range imports.Hoisted {
				if len(bytes.TrimSpace(hoisted)) == 0 {
					continue
				}
				hoistedLoc := imports.HoistedLocs[i]

				// trim leading whitespace
				// for the first import
				// if i == 0 {
				// 	hoisted = bytes.TrimLeft(hoisted, whitespace)
				// }
				p.printTextWithSourcemap(string(hoisted), loc.Loc{Start: start + hoistedLoc.Start})
			}

			p.addNilSourceMapping()
			p.printCSSImports(opts.cssLen)

			// 1. Component imports, if any exist.
			p.addNilSourceMapping()
			p.printComponentMetadata(n.Parent, opts.opts, []byte(p.sourcetext))

			// 2. Top-level Astro global.
			if printAstroGlobal {
				p.printTopLevelAstro(opts.opts)
			}

			// PRINT EXPORTS
			for i, exported := range exports {
				exportLoc := exportLocs[i]
				if len(bytes.TrimSpace(exported)) == 0 {
					continue
				}
				p.printTextWithSourcemap(string(bytes.TrimSpace(exported)), exportLoc)
				p.addNilSourceMapping()
				p.println("")
			}

			p.printFuncPrelude(opts.opts, printAstroGlobal)
			// PRINT BODY
			for i, body := range bodies {
				bodyLoc := bodiesLocs[i]
				if len(bytes.TrimSpace(body)) == 0 {
					continue
				}
				p.printTextWithSourcemap(string(body), bodyLoc)
			}
			// Print empty just to ensure a newline
			p.println("")
			p.println("")
			if len(n.Parent.Styles) > 0 {
				definedVars := transform.GetDefineVars(n.Parent.Styles)
				if len(definedVars) > 0 {
					p.printf("const $$definedVars = %s([%s]);\n", DEFINE_STYLE_VARS, strings.Join(definedVars, ","))
				}
			}

			p.printReturnOpen()
		} else {
			render1(p, c, RenderOptions{
				isRoot:           false,
				isExpression:     true,
				depth:            depth + 1,
				opts:             opts.opts,
				cssLen:           opts.cssLen,
				printedMaybeHead: opts.printedMaybeHead,
				scriptCount:      opts.scriptCount,
			})
			if len(n.Loc) > 1 {
				p.addSourceMapping(loc.Loc{Start: n.Loc[1].Start - 3})
			}
		}
		return
	} else if !p.hasFuncPrelude {
		p.printComponentMetadata(n.Parent, opts.opts, []byte{})
		if printAstroGlobal {
			p.printTopLevelAstro(opts.opts)
		}

		// Render func prelude. Will only run for the first non-frontmatter node
		p.printFuncPrelude(opts.opts, printAstroGlobal)
		// This just ensures a newline
		p.println("")

		// If we haven't printed the funcPrelude but we do have Styles/Scripts, we need to print them!
		if len(n.Parent.Styles) > 0 {
			definedVars := transform.GetDefineVars(n.Parent.Styles)
			if len(definedVars) > 0 {
				p.printf("const $$definedVars = %s([%s]);\n", DEFINE_STYLE_VARS, strings.Join(definedVars, ","))
			}
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
		p.printTextWithSourcemap(text, n.Loc[0])
		return
	case ElementNode:
		// No-op.
	case CommentNode:
		start := n.Loc[0].Start - 4
		p.addSourceMapping(loc.Loc{Start: start})
		p.print("<!--")
		start += 4
		p.addSourceMapping(loc.Loc{Start: start})
		p.printTextWithSourcemap(escapeText(n.Data), n.Loc[0])
		start += len(n.Data)
		p.addSourceMapping(loc.Loc{Start: start})
		p.print("-->")
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
		if n.FirstChild == nil || emptyTextNodeWithoutSiblings(n.FirstChild) {
			p.print("${(void 0)")
		} else if expressionOnlyHasComment(n) {
			// we do not print expressions that only contain comment blocks
			return
		} else {
			p.print("${")
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			p.addSourceMapping(c.Loc[0])
			if c.Type == TextNode {
				p.printTextWithSourcemap(c.Data, c.Loc[0])
				continue
			}
			// Print the opening of a tagged render function before
			// a node, only when it meets either of these conditions:
			// - It does not have a previous sibling.
			// - It has a text node that contains more than just whitespace.
			// - It is the first child of its parent expression.
			if c.PrevSibling == nil || c.PrevSibling == n.FirstChild || (c.PrevSibling.Type == TextNode && strings.TrimSpace(c.PrevSibling.Data) != "") {
				p.printTemplateLiteralOpen()
			}
			render1(p, c, RenderOptions{
				isRoot:           false,
				isExpression:     true,
				depth:            depth + 1,
				opts:             opts.opts,
				cssLen:           opts.cssLen,
				printedMaybeHead: opts.printedMaybeHead,
				scriptCount:      opts.scriptCount,
			})

			// Print the closing of a tagged render function after
			// a node, only when it meets either of these conditions:
			// - It does not have a next sibling.
			// - It has a text node that contains more than just whitespace.
			// - It is the last child of its parent expression.
			if c.NextSibling == nil || c.NextSibling == n.LastChild || (c.NextSibling.Type == TextNode && strings.TrimSpace(c.NextSibling.Data) != "") {
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
	isHandledScript := n.HandledScript
	for _, a := range n.Attr {
		if isSlot && a.Key == "is:inline" {
			isSlot = false
		}
		if transform.IsImplicitNodeMarker(a) {
			isImplicit = true
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
	case isHandledScript:
		// import '/src/pages/index.astro?astro&type=script&index=0&lang.ts';
		scriptUrl := fmt.Sprintf("%s?astro&type=script&index=%v&lang.ts", p.opts.Filename, *opts.scriptCount)
		resolvedScriptUrl := transform.ResolveIdForMatch(scriptUrl, &p.opts)
		escapedScriptUrl := escapeDoubleQuote(resolvedScriptUrl)
		p.print(fmt.Sprintf("${%s(%s,\"%s\")}", RENDER_SCRIPT, RESULT, escapedScriptUrl))
		*opts.scriptCount++
		return
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

	p.addSourceMapping(n.Loc[0])
	switch true {
	case isFragment:
		p.print("Fragment")
	case isClientOnly:
		p.print("null")
	case !isSlot && n.CustomElement:
		p.print(fmt.Sprintf("'%s'", n.Data))
	case !isSlot && !isImplicit:
		// Print the tag name
		p.print(n.Data)
	}

	p.addNilSourceMapping()
	if isImplicit {
		// do nothing
	} else if isComponent {
		maybeConvertTransition(n)
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
		maybeConvertTransition(n)

		for _, a := range n.Attr {
			if transform.IsImplicitNodeMarker(a) || a.Key == "is:inline" {
				continue
			}

			if a.Key == "slot" {
				if n.Parent.Component || n.Parent.Expression {
					continue
				}
				// Note: if we encounter "slot" NOT inside a component, that's fine
				// These should be preserved in the output
				p.printAttribute(a, n)
			} else if a.Key == "data-astro-source-file" {
				p.printAttribute(a, n)
				var l []int
				if n.FirstChild != nil && len(n.FirstChild.Loc) > 0 {
					start := n.FirstChild.Loc[0].Start
					if n.FirstChild.Type == TextNode {
						start += len(n.Data) - len(strings.TrimLeftFunc(n.Data, unicode.IsSpace))
					}
					l = p.builder.GetLineAndColumnForLocation(loc.Loc{Start: start})
				} else if len(n.Loc) > 0 {
					l = p.builder.GetLineAndColumnForLocation(n.Loc[0])
				}
				if len(l) > 0 {
					p.printAttribute(Attribute{
						Key:  "data-astro-source-loc",
						Type: QuotedAttribute,
						Val:  fmt.Sprintf("%d:%d", l[0], l[1]),
					}, n)
				}
				p.addSourceMapping(n.Loc[0])
			} else {
				p.printAttribute(a, n)
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
		p.printDefineVarsOpen(n)
	}

	// Render any child nodes.
	switch n.Data {
	case "iframe", "noembed", "noframes", "noscript", "plaintext", "script", "style", "xmp":
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == TextNode {
				p.printTextWithSourcemap(escapeText(c.Data), c.Loc[0])
			} else {
				render1(p, c, RenderOptions{
					isRoot:           false,
					isExpression:     opts.isExpression,
					depth:            depth + 1,
					opts:             opts.opts,
					cssLen:           opts.cssLen,
					printedMaybeHead: opts.printedMaybeHead,
					scriptCount:      opts.scriptCount,
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
						scriptCount:      opts.scriptCount,
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

					// Only slot ElementNodes (except expressions containing only comments) or non-empty TextNodes!
					// CommentNode, JSX comments and others should not be slotted
					if expressionOnlyHasComment(c) {
						continue
					}
					if c.Type == ElementNode || c.Type == TextNode && !emptyTextNodeWithoutSiblings(c) {
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
				numberOfSlots := len(slottedKeys)
				if numberOfSlots > 0 {
				childrenLoop:
					for _, slotProp := range slottedKeys {
						children := slottedChildren[slotProp]

						// If there are named slots, the default slot cannot be only whitespace
						if numberOfSlots > 1 && slotProp == "\"default\"" {
							// Loop over the children and verify that at least one non-whitespace node exists.
							foundNonWhitespace := false
							for _, child := range children {
								if child.Type != TextNode || strings.TrimSpace(child.Data) != "" {
									foundNonWhitespace = true
								}
							}
							if !foundNonWhitespace {
								continue childrenLoop
							}
						}

						// If selected, pass through result object on the Astro side
						if opts.opts.ResultScopedSlot {
							p.print(fmt.Sprintf(`%s: %s($$result) => `, slotProp, p.getAsyncFuncPrefix()))
						} else {
							p.print(fmt.Sprintf(`%s: %s() => `, slotProp, p.getAsyncFuncPrefix()))
						}

						p.printTemplateLiteralOpen()
						for _, child := range children {
							render1(p, child, RenderOptions{
								isRoot:           false,
								isExpression:     opts.isExpression,
								depth:            depth + 1,
								opts:             opts.opts,
								cssLen:           opts.cssLen,
								printedMaybeHead: opts.printedMaybeHead,
								scriptCount:      opts.scriptCount,
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
								scriptCount:      opts.scriptCount,
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
						scriptCount:      opts.scriptCount,
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
						scriptCount:      opts.scriptCount,
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
		start := 2
		if len(n.Loc) > 0 {
			start = n.Loc[0].Start
		}
		if len(n.Loc) >= 2 {
			start = n.Loc[1].Start
		}
		start -= 2
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
// nolint
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
