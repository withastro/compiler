// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package printer

import (
	"fmt"
	"regexp"
	"strings"

	. "github.com/snowpackjs/astro/internal"
	"github.com/snowpackjs/astro/internal/js_scanner"
	"github.com/snowpackjs/astro/internal/loc"
	"github.com/snowpackjs/astro/internal/sourcemap"
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
func PrintToJS(sourcetext string, n *Node) PrintResult {
	p := &printer{
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
		importStatements := make([]ExtractedStatement, 0)
		frontmatterStatements := make([]ExtractedStatement, 0)

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == TextNode {
				offset := c.Loc[0].Start - n.Loc[0].Start
				imports := js_scanner.FindImportStatements([]byte(c.Data))
				var prevImport *js_scanner.ImportStatement
				for i, currImport := range imports {
					var nextImport *js_scanner.ImportStatement
					if i < len(imports)-1 {
						nextImport = imports[i+1]
					}
					// Extract import statement
					importStatements = append(importStatements, ExtractedStatement{
						Loc:     loc.Loc{Start: offset + currImport.StatementStart},
						Content: c.Data[currImport.StatementStart:currImport.StatementEnd] + "\n",
					})
					if i == 0 {
						content := c.Data[0:currImport.StatementStart]
						if strings.TrimSpace(content) != "" {
							frontmatterStatements = append(frontmatterStatements, ExtractedStatement{
								Loc:     loc.Loc{Start: offset + 1},
								Content: content,
							})
						}
					}
					if prevImport != nil {
						content := c.Data[prevImport.StatementEnd:currImport.StatementStart]
						if strings.TrimSpace(content) != "" {
							frontmatterStatements = append(frontmatterStatements, ExtractedStatement{
								Loc:     loc.Loc{Start: offset + prevImport.StatementEnd + 1},
								Content: content,
							})
						}
					}
					if nextImport != nil {
						content := c.Data[currImport.StatementEnd:nextImport.StatementStart]
						if strings.TrimSpace(content) != "" {
							frontmatterStatements = append(frontmatterStatements, ExtractedStatement{
								Loc:     loc.Loc{Start: offset + currImport.StatementEnd + 1},
								Content: content,
							})
						}
					}
					if i == len(imports)-1 {
						content := c.Data[currImport.StatementEnd:]
						if strings.TrimSpace(content) != "" {
							frontmatterStatements = append(frontmatterStatements, ExtractedStatement{
								Loc:     loc.Loc{Start: offset + currImport.StatementEnd + 1},
								Content: content,
							})
						}
					}
					prevImport = currImport
				}
				for _, statement := range importStatements {
					p.addSourceMapping(statement.Loc)
					p.print(statement.Content)
				}
				// TODO: use the proper component name
				p.printFuncPrelude("Component")

				if len(frontmatterStatements) > 0 || len(importStatements) == 0 {
					p.addSourceMapping(n.Loc[0])
					p.println("// ---")
					if len(frontmatterStatements) > 0 {
						for _, statement := range frontmatterStatements {
							p.addSourceMapping(statement.Loc)
							p.print(strings.TrimLeft(statement.Content, " \t\r\n"))
						}
					} else if len(importStatements) == 0 {
						p.addSourceMapping(c.Loc[0])
						p.print(c.Data)
					}
					p.addSourceMapping(loc.Loc{Start: 0})
					p.println("// ---")
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
		p.printFuncPrelude("Component")
		p.printReturnOpen()
	}
	switch n.Type {
	case TextNode:
		if strings.TrimSpace(n.Data) == "" {
			p.addSourceMapping(n.Loc[0])
			p.print(n.Data)
			return
		}
		backticks := regexp.MustCompile("`")
		dollarOpen := regexp.MustCompile(`\${`)
		text := n.Data
		text = strings.Replace(text, "\\", "\\\\", -1)
		text = backticks.ReplaceAllString(text, "\\`")
		text = dollarOpen.ReplaceAllString(text, "\\${")
		p.addSourceMapping(n.Loc[0])
		p.print(text)
		return
	case ElementNode:
		// No-op.
	case CommentNode:
		p.addSourceMapping(n.Loc[0])
		p.print("<!--")
		p.print(n.Data)
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
			if c.Type == TextNode {
				p.addSourceMapping(c.Loc[0])
				p.print(c.Data)
			} else {
				p.addSourceMapping(c.Loc[0])
				if (c.PrevSibling == nil || c.PrevSibling != nil && c.PrevSibling.Type == TextNode) && c.NextSibling != nil && c.NextSibling.Type != TextNode {
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

	p.addSourceMapping(n.Loc[0])
	if isComponent {
		p.print("${renderComponent(")
	} else {
		p.print("<")
	}

	p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start + 1})

	// fmt.Println("OPEN", n.Data)

	if n.Fragment {
		p.print("Fragment")
	} else {
		p.print(n.Data)
	}

	p.addSourceMapping(n.Loc[0])
	if isComponent {
		if len(n.Attr) != 0 {
			p.print(", {")
		} else {
			p.print(", null")
		}
		for i, a := range n.Attr {
			if i != 0 {
				p.print(",")
			}
			switch a.Type {
			case QuotedAttribute:
				p.addSourceMapping(a.KeyLoc)
				p.print(`"` + a.Key + `"`)
				p.print(":")
				p.addSourceMapping(a.ValLoc)
				p.print(`"` + a.Val + `"`)
			case EmptyAttribute:
				p.addSourceMapping(a.KeyLoc)
				p.print(`"` + a.Key + `"`)
				p.print(":")
				p.print("true")
			case ExpressionAttribute:
				p.addSourceMapping(a.KeyLoc)
				p.print(`"` + a.Key + `"`)
				p.print(":")
				p.addSourceMapping(a.ValLoc)
				p.print(`(` + a.Val + `)`)
			case SpreadAttribute:
				p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start - 3})
				p.print(`...(` + strings.TrimSpace(a.Key) + `)`)
			case ShorthandAttribute:
				p.addSourceMapping(a.KeyLoc)
				p.print(`"` + strings.TrimSpace(a.Key) + `"`)
				p.print(":")
				p.addSourceMapping(a.KeyLoc)
				p.print(`(` + strings.TrimSpace(a.Key) + `)`)
			case TemplateLiteralAttribute:
				p.addSourceMapping(a.KeyLoc)
				p.print(`"` + strings.TrimSpace(a.Key) + `"`)
				p.print(":")
				p.print("`" + strings.TrimSpace(a.Key) + "`")
			}
			p.addSourceMapping(n.Loc[0])
			// if i != len(n.Attr)-1 {
			// 	p.print("")
			// }
		}
		if len(n.Attr) != 0 {
			p.print("}")
		}
	} else {
		for _, a := range n.Attr {
			if a.Namespace != "" {
				p.print(a.Namespace)
				p.print(":")
			}

			switch a.Type {
			case QuotedAttribute:
				p.print(" ")
				p.addSourceMapping(a.KeyLoc)
				p.print(a.Key)
				p.print("=")
				p.addSourceMapping(a.ValLoc)
				p.print(`"` + a.Val + `"`)
			case EmptyAttribute:
				p.print(" ")
				p.addSourceMapping(a.KeyLoc)
				p.print(a.Key)
			case ExpressionAttribute:
				p.print("${addAttribute(")
				p.addSourceMapping(a.ValLoc)
				p.print(strings.TrimSpace(a.Val))
				p.addSourceMapping(a.KeyLoc)
				p.print(`, "` + strings.TrimSpace(a.Key) + `")}`)
			case SpreadAttribute:
				p.print("${spreadAttributes(")
				p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start - 3})
				p.print(strings.TrimSpace(a.Key))
				p.print(`, "` + strings.TrimSpace(a.Key) + `")}`)
			case ShorthandAttribute:
				p.print("${addAttribute(")
				p.addSourceMapping(a.KeyLoc)
				p.print(strings.TrimSpace(a.Key))
				p.addSourceMapping(a.KeyLoc)
				p.print(`, "` + strings.TrimSpace(a.Key) + `")}`)
			case TemplateLiteralAttribute:
				p.print("${addAttribute(`")
				p.addSourceMapping(a.ValLoc)
				p.print(strings.TrimSpace(a.Val))
				p.addSourceMapping(a.KeyLoc)
				p.print("`" + `, "` + strings.TrimSpace(a.Key) + `")}`)
			}
			p.addSourceMapping(n.Loc[0])
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
				p.print(c.Data)
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
		if isComponent {
			p.print(", ")
			p.printTemplateLiteralOpen()
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			render1(p, c, RenderOptions{
				isRoot:       false,
				isExpression: opts.isExpression,
				depth:        depth + 1,
			})
		}
		if isComponent {
			p.printTemplateLiteralClose()
		}
	}

	if len(n.Loc) == 2 {
		p.addSourceMapping(n.Loc[1])
	} else {
		p.addSourceMapping(n.Loc[0])
	}
	if isComponent {
		p.print(")}")
	} else {
		p.print(`</` + n.Data + `>`)
	}

	if opts.isRoot {
		p.printReturnClose()
		// TODO: use proper component name
		p.printFuncSuffix("Component")
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
