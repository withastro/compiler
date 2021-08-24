// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package printer

import (
	"regexp"
	"strings"

	. "github.com/snowpackjs/astro/internal"
	"github.com/snowpackjs/astro/internal/js_scanner"
	"github.com/snowpackjs/astro/internal/loc"
	"github.com/snowpackjs/astro/internal/sourcemap"
	a "golang.org/x/net/html/atom"
)

func (p *printer) print(text string) {
	p.output = append(p.output, text...)
}

// This is the same as "print(string(bytes))" without any unnecessary temporary
// allocations
func (p *printer) printBytes(bytes []byte) {
	p.output = append(p.output, bytes...)
}

func (p *printer) addSourceMapping(location loc.Loc) {
	p.builder.AddSourceMapping(location, p.output)
}

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
	sources := make([]string, 1)
	sources[0] = "test.astro"
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
	// Render non-element nodes; these are the easy cases.
	switch n.Type {
	case TextNode:
		backticks := regexp.MustCompile("`")
		dollarOpen := regexp.MustCompile(`\${`)
		text := n.Data
		text = strings.Replace(text, "\\", "\\\\", -1)
		text = backticks.ReplaceAllString(text, "\\`")
		text = dollarOpen.ReplaceAllString(text, "\\${")
		p.print(",")
		p.addSourceMapping(n.Loc[0])
		p.print("`" + text + "`")
		return
	case FrontmatterNode:
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
				p.addSourceMapping(loc.Loc{Start: 0})
				p.print("//@ts-ignore\nasync function __render(props, ...children) {\n")
				p.addSourceMapping(n.Loc[0])
				p.print("// ---")
				for _, statement := range frontmatterStatements {
					p.addSourceMapping(statement.Loc)
					p.print(statement.Content)
				}
			} else {
				render1(p, c, RenderOptions{
					isRoot:       false,
					isExpression: true,
					depth:        depth + 1,
				})
			}
		}
		p.addSourceMapping(loc.Loc{Start: n.Loc[1].Start - 3})
		p.print("// ---")
		return
	case DocumentNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			render1(p, c, RenderOptions{
				isRoot:       true,
				isExpression: false,
				depth:        depth + 1,
			})
		}
		return
	case ElementNode:
		// No-op.
	case CommentNode:
		p.print(",")
		p.addSourceMapping(n.Loc[0])
		p.print("`<!--")
		p.print(n.Data)
		p.print("-->`")
		return
	case DoctypeNode:
		p.print("<!DOCTYPE ")
		p.print(n.Data)
		// if n.Attr != nil {
		// 	var p, s string
		// 	for _, a := range n.Attr {
		// 		switch a.Key {
		// 		case "public":
		// 			p = a.Val
		// 		case "system":
		// 			s = a.Val
		// 		}
		// 	}
		// 	if p != "" {
		// 		if _, err := w.WriteString(" PUBLIC "); err != nil {
		// 			return err
		// 		}
		// 		if err := writeQuoted(w, p); err != nil {
		// 			return err
		// 		}
		// 		if s != "" {
		// 			if err := w.WriteByte(' '); err != nil {
		// 				return err
		// 			}
		// 			if err := writeQuoted(w, s); err != nil {
		// 				return err
		// 			}
		// 		}
		// 	} else if s != "" {
		// 		if _, err := w.WriteString(" SYSTEM "); err != nil {
		// 			return err
		// 		}
		// 		if err := writeQuoted(w, s); err != nil {
		// 			return err
		// 		}
		// 	}
		// }
		p.printBytes([]byte{','})
		return
	case RawNode:
		p.print(n.Data)
		return
	}

	// Tip! Comment this block out to debug expressions
	if n.Expression {
		if n.FirstChild != nil {
			p.print(",(")
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == TextNode {
				p.addSourceMapping(c.Loc[0])
				p.print(c.Data)
			} else {
				p.addSourceMapping(c.Loc[0])
				if (c.PrevSibling == nil || c.PrevSibling != nil && c.PrevSibling.Type == TextNode) && c.NextSibling != nil && c.NextSibling.Type != TextNode {
					p.print("h(Fragment,null,")
				}
				if c.PrevSibling != nil && c.PrevSibling.Type != TextNode {
					p.print(",")
				}
				render1(p, c, RenderOptions{
					isRoot:       false,
					isExpression: true,
					depth:        depth + 1,
				})
				if c.NextSibling == nil || (c.NextSibling != nil && c.NextSibling.Type == TextNode) {
					p.print(")")
				}
			}
		}
		p.print(")")
		p.addSourceMapping(n.Loc[1])
		return
	}

	isComponent := (n.Component || n.CustomElement) && n.Data != "Fragment"

	if opts.isRoot {
		p.print("\nreturn h(Fragment,null,")
	} else if !opts.isExpression {
		p.print(",")
	}

	p.addSourceMapping(n.Loc[0])
	if isComponent {
		p.print("h(render_component(")
	} else {
		p.print("h(")
	}

	p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start + 1})
	if n.Component {
		p.print(n.Data)
	} else if n.Fragment {
		p.print("Fragment")
	} else {
		p.print(`"` + n.Data + `"`)
	}
	p.print(",")

	p.addSourceMapping(n.Loc[0])
	if len(n.Attr) == 0 {
		p.print("null")
	} else {
		p.print("{")
	}
	for i, a := range n.Attr {
		if a.Namespace != "" {
			p.print(a.Namespace)
			p.print(":")
		}

		switch a.Type {
		case QuotedAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(`"` + a.Key + `"`)
			p.print(":")
			p.addSourceMapping(a.ValLoc)
			p.print(`"` + a.Val + `"`)
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
		if i != len(n.Attr)-1 {
			p.print(",")
		}
	}
	if len(n.Attr) != 0 {
		p.print("}")
	}

	if voidElements[n.Data] {
		//nolint
		if n.FirstChild != nil {
			// return fmt.Errorf("html: void element <%s> has child nodes", n.Data)
		}
		p.addSourceMapping(n.Loc[0])
		p.print(")")
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
				p.print(",`")
				p.print(c.Data)
				p.print("`")
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
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			render1(p, c, RenderOptions{
				isRoot:       false,
				isExpression: opts.isExpression,
				depth:        depth + 1,
			})
		}
	}

	if n.DataAtom == a.Html {
		p.print(`,'\n'`)
	}
	if len(n.Loc) == 2 {
		p.addSourceMapping(n.Loc[1])
	} else {
		p.addSourceMapping(n.Loc[0])
	}
	p.print(`)`)
	if isComponent {
		p.print(`)`)
	}

	if opts.isRoot {
		p.addSourceMapping(loc.Loc{Start: 0})
		p.print(")\n}\n")
		p.print("\n\nexport default { isAstroComponent: true, __render }\n")
	}
}

// Section 12.1.2, "Elements", gives this list of void elements. Void elements
// are those that can't have any contents.
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
