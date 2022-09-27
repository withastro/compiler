package printer

import (
	"fmt"
	"strings"

	. "github.com/withastro/compiler/internal"
	astro "github.com/withastro/compiler/internal"
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
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderTsx(p, c)
		}
		p.addNilSourceMapping()
		p.print("\n</Fragment>")
		propType := "Record<string, any>"
		if p.hasTypedProps {
			propType = "Props"
		}
		componentName := getTSXComponentName(p.opts.Filename)
		p.print(fmt.Sprintf("\n\nexport default function %s(_props: %s): any {}\n", componentName, propType))
		return
	}

	if n.Type == FrontmatterNode {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == TextNode {
				if strings.Contains(c.Data, "Props") {
					p.hasTypedProps = true
				}
				if len(c.Loc) > 0 {
					p.addSourceMapping(c.Loc[0])
				}
				p.printTextWithSourcemap(c.Data, c.Loc[0])
			} else {
				renderTsx(p, c)
			}
		}
		if n.FirstChild != nil {
			// Convert closing `---` to a `\n`, just in case
			p.addNilSourceMapping()
			p.println("\n")
		}

		p.addNilSourceMapping()
		p.print(";<Fragment>\n")
		return
	}

	switch n.Type {
	case TextNode:
		switch getTextType(n) {
		case ScriptText:
			p.addNilSourceMapping()
			p.print("{() => {")
			p.printTextWithSourcemap(n.Data, n.Loc[0])
			p.addNilSourceMapping()
			p.print("}}")
			return
		}
		if strings.TrimSpace(n.Data) == "" {
			p.printTextWithSourcemap(n.Data, n.Loc[0])
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
	for _, a := range n.Attr {
		if isInvalidTSXAttributeName(a.Key) {
			invalidTSXAttributes = append(invalidTSXAttributes, a)
			continue
		}
		p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start - 1})
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
			p.addSourceMapping(a.ValLoc)
			p.print(`"` + encodeDoubleQuote(a.Val) + `"`)
		case astro.EmptyAttribute:
			p.print(a.Key)
		case astro.ExpressionAttribute:
			p.print(a.Key)
			p.addSourceMapping(loc.Loc{Start: eqStart})
			p.print(`=`)
			p.addSourceMapping(loc.Loc{Start: eqStart + 1})
			p.print(`{`)
			p.printTextWithSourcemap(a.Val, loc.Loc{Start: eqStart + 2})
			p.addSourceMapping(loc.Loc{Start: eqStart + 2 + len(a.Val)})
			p.print(`}`)
		case astro.SpreadAttribute:
			p.print(a.Key)
			p.addSourceMapping(loc.Loc{Start: eqStart})
			p.print(`=`)
			p.addSourceMapping(loc.Loc{Start: eqStart + 1})
			p.addSourceMapping(a.ValLoc)
			p.print(fmt.Sprintf(`{...%s}`, a.Val))
		case astro.ShorthandAttribute:
			withoutComments := removeComments(a.Key)
			if len(withoutComments) == 0 {
				return
			}
			p.print(a.Key)
			p.print(`=`)
			p.print(fmt.Sprintf(`{%s}`, a.Key))
		case astro.TemplateLiteralAttribute:
			p.print(a.Key)
			p.addSourceMapping(loc.Loc{Start: eqStart})
			p.print(`=`)
			p.addSourceMapping(loc.Loc{Start: eqStart + 1})
			p.printTextWithSourcemap(fmt.Sprintf("{`%s`}", escapeText(a.Val)), a.ValLoc)
		}
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
			p.addSourceMapping(loc.Loc{Start: eqStart + 1 + len(a.Val) + 2})
		case astro.EmptyAttribute:
			p.print(a.Key)
			p.print(`"`)
			p.addSourceMapping(loc.Loc{Start: eqStart})
			p.print(`:`)
			p.addSourceMapping(a.KeyLoc)
			p.print(`true`)
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
		case astro.SpreadAttribute:
			p.addSourceMapping(loc.Loc{Start: eqStart})
			p.print("=")
			p.addSourceMapping(a.ValLoc)
			p.print(fmt.Sprintf(`...%s`, a.Val))
		case astro.ShorthandAttribute:
			withoutComments := removeComments(a.Key)
			if len(withoutComments) == 0 {
				return
			}
			p.addSourceMapping(a.KeyLoc)
			p.print(a.Key)
		case astro.TemplateLiteralAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(a.Key)
			p.print(`":`)
			p.addSourceMapping(a.ValLoc)
			p.print(fmt.Sprintf("`%s`", escapeText(a.Val)))
		}
		if i == len(invalidTSXAttributes)-1 {
			p.addNilSourceMapping()
			p.print("}}")
		}
	}
	if voidElements[n.Data] && n.FirstChild == nil {
		p.addSourceMapping(n.Loc[0])
		p.print("/>")
		return
	}
	if len(n.Attr) > 0 {
		start := n.Attr[len(n.Attr)-1].ValLoc.Start + len(n.Attr[len(n.Attr)-1].Val)
		offset := strings.IndexRune(p.sourcetext[start:], '>')
		start += offset
		p.addSourceMapping(loc.Loc{Start: start})
	} else {
		p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start + len(n.Data)})
	}
	p.print(">")

	// Render any child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderTsx(p, c)
	}
	endLoc := n.Loc[0].Start
	if len(n.Loc) == 2 {
		endLoc = n.Loc[1].Start
	}
	p.addSourceMapping(loc.Loc{Start: endLoc - 2})
	p.print("</")
	p.addSourceMapping(loc.Loc{Start: endLoc})
	p.print(n.Data)
	p.addSourceMapping(loc.Loc{Start: endLoc + len(n.Data)})
	p.print(">")
}
