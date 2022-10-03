package printer

import (
	"fmt"
	"strings"
	"unicode"

	. "github.com/withastro/compiler/internal"
	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/js_scanner"
	"github.com/withastro/compiler/internal/loc"
	"github.com/withastro/compiler/internal/sourcemap"
	"github.com/withastro/compiler/internal/transform"
	"golang.org/x/net/html/atom"
)

func PrintToTSX(sourcetext string, n *Node, opts transform.TransformOptions) PrintResult {
	p := &printer{
		sourcetext: sourcetext,
		opts:       opts,
		builder:    sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(sourcetext, len(strings.Split(sourcetext, "\n")))),
	}

	renderTsx(p, n)
	return PrintResult{
		Output:         p.output,
		SourceMapChunk: p.builder.GenerateChunk(p.output),
	}
}

func isScript(p *astro.Node) bool {
	return p.DataAtom == atom.Script
}

var ScriptMimeTypes map[string]bool = map[string]bool{
	"module":                 true,
	"text/typescript":        true,
	"application/javascript": true,
	"text/partytown":         true,
	"application/node":       true,
}

func isInvalidTSXAttributeName(k string) bool {
	return strings.HasPrefix(k, "@") || strings.Contains(k, ".")
}

type TextType uint32

const (
	RawText TextType = iota
	ScriptText
)

func getTextType(n *astro.Node) TextType {
	if script := n.Closest(isScript); script != nil {
		attr := astro.GetAttribute(script, "type")
		if attr == nil || (attr != nil && ScriptMimeTypes[strings.ToLower(attr.Val)]) {
			return ScriptText
		}
	}
	return RawText
}

func renderTsx(p *printer, n *Node) {
	// Root of the document, print all children
	if n.Type == DocumentNode {
		hasChildren := false
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			// This checks for the first node that comes *after* the frontmatter
			// to ensure that the statement is properly closed with a `;`.
			// Without this, TypeScript can get tripped up by the body of our file.
			if c.PrevSibling != nil && c.PrevSibling.Type == FrontmatterNode {
				buf := strings.TrimSpace(string(p.output))
				if len(buf) > 1 {
					char := rune(buf[len(buf)-1:][0])
					// If the existing buffer ends with a punctuation character, we need a `;`
					if !unicode.IsLetter(char) && char != ';' {
						p.addNilSourceMapping()
						p.print(";")
					}
				}
				// We always need to start the body with `<Fragment>`
				p.addNilSourceMapping()
				p.print("<Fragment>\n")
				hasChildren = true
			}
			if c.PrevSibling == nil && c.Type != FrontmatterNode {
				p.addNilSourceMapping()
				p.print("<Fragment>\n")
				hasChildren = true
			}
			renderTsx(p, c)
		}
		p.addSourceMapping(loc.Loc{Start: len(p.sourcetext)})
		p.print("\n")

		p.addNilSourceMapping()
		// Only close the body with `</Fragment>` if we printed a body
		if hasChildren {
			p.print("</Fragment>\n")
		}
		props := js_scanner.GetPropsType(p.output)
		componentName := getTSXComponentName(p.opts.Filename)

		p.print(fmt.Sprintf("export default function %s%s(_props: %s%s): any {}", componentName, props.Statement, props.Ident, props.Generics))
		return
	}

	if n.Type == FrontmatterNode {
		p.addSourceMapping(loc.Loc{Start: 0})
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == TextNode {
				if len(c.Loc) > 0 {
					p.addSourceMapping(c.Loc[0])
				}
				p.printTextWithSourcemap(c.Data, c.Loc[0])
			} else {
				renderTsx(p, c)
			}
		}
		if n.FirstChild != nil {
			p.addSourceMapping(loc.Loc{Start: n.FirstChild.Loc[0].Start + len(n.FirstChild.Data)})
			p.print("")
			p.addSourceMapping(loc.Loc{Start: n.FirstChild.Loc[0].Start + len(n.FirstChild.Data) + 3})
			p.println("")
		}
		return
	}

	switch n.Type {
	case TextNode:
		if getTextType(n) == ScriptText {
			p.addNilSourceMapping()
			p.print("\n{() => {")
			p.printTextWithSourcemap(n.Data, n.Loc[0])
			p.addNilSourceMapping()
			p.print("}}\n")
			p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start + len(n.Data)})
			return
		} else if strings.ContainsAny(n.Data, "{}") {
			p.addNilSourceMapping()
			p.print("{`")
			p.printTextWithSourcemap(escapeText(n.Data), n.Loc[0])
			p.addNilSourceMapping()
			p.print("`}")
		} else {
			p.printTextWithSourcemap(n.Data, n.Loc[0])
		}
		return
	case ElementNode:
		// No-op.
	case CommentNode:
		// p.addSourceMapping(n.Loc[0])
		p.addNilSourceMapping()
		p.print("{/**")
		p.addSourceMapping(n.Loc[0])
		p.printTextWithSourcemap(escapeBraces(n.Data), n.Loc[0])
		p.addNilSourceMapping()
		p.print("*/}")
		return
	default:
		return
	}

	if n.Expression {
		p.addSourceMapping(n.Loc[0])
		if n.FirstChild == nil {
			p.print("{(void 0)")
		} else if expressionOnlyHasCommentBlock(n) {
			// we do not print expressions that only contain comment blocks
			return
		} else {
			p.print("{")
		}
		start := n.Loc[0].Start + 1
		p.addSourceMapping(loc.Loc{Start: start})

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == TextNode {
				if c == n.FirstChild {
					p.printTextWithSourcemap(c.Data, loc.Loc{Start: start})
				} else {
					p.printTextWithSourcemap(c.Data, c.Loc[0])
				}
				continue
			}
			if c.PrevSibling == nil || c.PrevSibling.Type == TextNode {
				p.addNilSourceMapping()
				p.print(`<Fragment>`)
			}
			renderTsx(p, c)
			if c.NextSibling == nil || c.NextSibling.Type == TextNode {
				p.addNilSourceMapping()
				p.print(`</Fragment>`)
			}
		}
		if len(n.Loc) == 2 {
			p.addSourceMapping(n.Loc[1])
		} else {
			p.addSourceMapping(n.Loc[0])
		}
		p.print("}")
		return
	}

	isImplicit := false
	for _, a := range n.Attr {
		if transform.IsImplictNodeMarker(a) {
			isImplicit = true
			break
		}
	}

	if isImplicit {
		// Render any child nodes
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderTsx(p, c)
		}
		return
	}

	p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start - 1})
	p.print("<")
	p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start})
	p.print(n.Data)

	invalidTSXAttributes := make([]Attribute, 0)
	endLoc := n.Loc[0].Start + len(n.Data)
	for _, a := range n.Attr {
		if isInvalidTSXAttributeName(a.Key) {
			invalidTSXAttributes = append(invalidTSXAttributes, a)
			continue
		}
		offset := 1
		if a.Type == astro.ShorthandAttribute {
			offset = 2
		}
		p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start - offset})
		p.print(" ")
		eqStart := a.KeyLoc.Start + strings.IndexRune(p.sourcetext[a.KeyLoc.Start:], '=')
		p.addSourceMapping(a.KeyLoc)
		if a.Namespace != "" {
			p.print(a.Namespace)
			p.print(":")
		}
		switch a.Type {
		case astro.QuotedAttribute:
			p.print(a.Key)
			p.addSourceMapping(loc.Loc{Start: eqStart})
			p.print("=")
			p.addSourceMapping(loc.Loc{Start: eqStart + 1})
			p.print(`"` + encodeDoubleQuote(a.Val) + `"`)
			endLoc = a.ValLoc.Start + len(a.Val)
		case astro.EmptyAttribute:
			p.print(a.Key)
			endLoc = a.KeyLoc.Start + len(a.Key)
		case astro.ExpressionAttribute:
			p.print(a.Key)
			p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start - 1})
			p.print(`=`)
			p.addSourceMapping(loc.Loc{Start: eqStart + 1})
			p.print(`{`)
			p.printTextWithSourcemap(a.Val, loc.Loc{Start: eqStart + 2})
			p.addSourceMapping(loc.Loc{Start: eqStart + 2 + len(a.Val)})
			p.print(`}`)
			endLoc = eqStart + len(a.Val) + 2
		case astro.SpreadAttribute:
			p.print(a.Key)
			p.addSourceMapping(loc.Loc{Start: eqStart})
			p.print(`=`)
			p.addSourceMapping(loc.Loc{Start: eqStart + 1})
			p.addSourceMapping(a.ValLoc)
			p.print(fmt.Sprintf(`{...%s}`, a.Val))
			endLoc = a.ValLoc.Start + len(a.Val) + 2
		case astro.ShorthandAttribute:
			withoutComments := removeComments(a.Key)
			if len(withoutComments) == 0 {
				return
			}
			p.print(a.Key)
			p.print(`=`)
			p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start - 1})
			p.print(`{`)
			p.addSourceMapping(a.KeyLoc)
			p.print(a.Key)
			p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start + len(a.Key)})
			p.print(`}`)
			endLoc = a.KeyLoc.Start + len(a.Key)
		case astro.TemplateLiteralAttribute:
			p.print(a.Key)
			p.addSourceMapping(loc.Loc{Start: eqStart})
			p.print(`=`)
			p.addSourceMapping(loc.Loc{Start: eqStart + 1})
			p.printTextWithSourcemap(fmt.Sprintf("{`%s`}", a.Val), a.ValLoc)
			endLoc = a.ValLoc.Start + len(a.Val) + 2
		}
		p.addSourceMapping(loc.Loc{Start: endLoc})
	}
	for i, a := range invalidTSXAttributes {
		if i == 0 {
			p.print(" {...{")
		} else {
			p.print(",")
		}
		eqStart := a.KeyLoc.Start + strings.IndexRune(p.sourcetext[a.KeyLoc.Start:], '=')
		p.addSourceMapping(a.KeyLoc)
		p.print(`"`)
		if a.Namespace != "" {
			p.print(a.Namespace)
			p.print(":")
		}
		switch a.Type {
		case astro.QuotedAttribute:
			p.print(a.Key)
			p.print(`"`)
			p.addSourceMapping(loc.Loc{Start: eqStart})
			p.print(`:`)
			p.addSourceMapping(loc.Loc{Start: eqStart + 1})
			p.print(`"` + encodeDoubleQuote(a.Val) + `"`)
			endLoc = eqStart + 1 + len(a.Val) + 2
		case astro.EmptyAttribute:
			p.print(a.Key)
			p.print(`"`)
			p.addNilSourceMapping()
			p.print(`:`)
			p.addSourceMapping(a.KeyLoc)
			p.print(`true`)
			endLoc = a.KeyLoc.Start + len(a.Key)
		case astro.ExpressionAttribute:
			p.print(a.Key)
			p.print(`"`)
			p.addSourceMapping(loc.Loc{Start: eqStart})
			p.print(`:`)
			p.addSourceMapping(loc.Loc{Start: eqStart + 1})
			p.print(`(`)
			p.printTextWithSourcemap(a.Val, loc.Loc{Start: eqStart + 2})
			p.addSourceMapping(loc.Loc{Start: eqStart + 2 + len(a.Val)})
			p.print(`)`)
			endLoc = eqStart + len(a.Val) + 2
		case astro.SpreadAttribute:
			p.addSourceMapping(a.ValLoc)
			p.print(fmt.Sprintf(`...%s`, a.Val))
			endLoc = a.ValLoc.Start + len(a.Val) + 3
		case astro.ShorthandAttribute:
			withoutComments := removeComments(a.Key)
			if len(withoutComments) == 0 {
				return
			}
			p.addSourceMapping(a.KeyLoc)
			p.print(a.Key)
			endLoc = a.KeyLoc.Start + len(a.Key)
		case astro.TemplateLiteralAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(a.Key)
			p.print(`":`)
			p.addSourceMapping(a.ValLoc)
			p.print(fmt.Sprintf("`%s`", a.Val))
			endLoc = a.ValLoc.Start + len(a.Val) + 2
		}
		if i == len(invalidTSXAttributes)-1 {
			p.addNilSourceMapping()
			p.print("}}")
		}
	}
	if len(n.Attr) == 0 {
		endLoc = n.Loc[0].Start + len(n.Data) - 1
	}
	isSelfClosing := false
	tmpLoc := endLoc
	for i := 0; i < len(p.sourcetext[tmpLoc:]); i++ {
		c := p.sourcetext[endLoc : endLoc+1][0]
		if c == '/' && p.sourcetext[endLoc+1:][0] == '>' {
			p.addSourceMapping(loc.Loc{Start: endLoc})
			isSelfClosing = true
			break
		} else if c == '>' {
			p.addSourceMapping(loc.Loc{Start: endLoc})
			endLoc++
			break
		} else {
			endLoc++
		}
	}

	if voidElements[n.Data] && n.FirstChild == nil {
		p.print("/>")
		return
	}
	if isSelfClosing && len(n.Attr) > 0 {
		p.print(" ")
		p.addSourceMapping(loc.Loc{Start: endLoc})
	}
	p.print(">")

	// Render any child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderTsx(p, c)
	}
	// Special case because of trailing expression close in scripts
	if n.DataAtom == atom.Script {
		p.printf("</%s>", n.Data)
		return
	}

	if len(n.Loc) > 1 {
		endLoc = n.Loc[1].Start - 2
	}
	p.addSourceMapping(loc.Loc{Start: endLoc})
	p.print("</")
	if !isSelfClosing {
		endLoc += 2
		p.addSourceMapping(loc.Loc{Start: endLoc})
	}
	p.print(n.Data)
	if !isSelfClosing {
		endLoc += len(n.Data)
		p.addSourceMapping(loc.Loc{Start: endLoc})
	}
	p.print(">")
	p.addSourceMapping(loc.Loc{Start: endLoc})
}
