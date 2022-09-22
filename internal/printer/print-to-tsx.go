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
		if attr != nil && ScriptMimeTypes[strings.ToLower(attr.Val)] {
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
				// if len(c.Loc) > 0 {
				// 	p.addSourceMapping(c.Loc[0])
				// }
				if n.LastChild.Data == c.Data {
					if !strings.HasSuffix(c.Data, ";\n") || !strings.HasSuffix(c.Data, ";") {
						c.Data = strings.TrimSuffix(c.Data, "\n")
						c.Data = "\n" + strings.TrimSpace(c.Data)
						if !strings.HasSuffix(c.Data, ";") {
							c.Data += ";\n"
						} else {
							c.Data += "\n"
						}
					}
				}
				p.print(c.Data)
			} else {
				renderTsx(p, c)
			}
		}
		p.print("<Fragment>\n")
		return
	}

	switch n.Type {
	case TextNode:
		if strings.TrimSpace(n.Data) == "" {
			// p.addSourceMapping(n.Loc[0])
			p.print(n.Data)
		} else if strings.ContainsAny(n.Data, "{}") {
			switch getTextType(n) {
			case RawText:
				p.print("{`")
				// p.addSourceMapping(n.Loc[0])
				p.print(escapeText(n.Data))
				p.print("`}")
			case ScriptText:
				p.print("{() => {")
				p.print(n.Data)
				// p.printTextWithSourcemap(n.Data, n.Loc[0])
				p.print("}}")
			}
		} else {
			p.printTextWithSourcemap(n.Data, n.Loc[0])
		}
		return
	case ElementNode:
		// No-op.
	case CommentNode:
		// p.addSourceMapping(n.Loc[0])
		p.print("{/**")
		p.print(escapeBraces(n.Data))
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
		p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start + 1})

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == TextNode {
				p.printTextWithSourcemap(c.Data, c.Loc[0])
				continue
			}
			if c.PrevSibling == nil || c.PrevSibling.Type == TextNode {
				p.print(`<Fragment>`)
			}
			renderTsx(p, c)
			if c.NextSibling == nil || c.NextSibling.Type == TextNode {
				p.print(`</Fragment>`)
			}
		}
		p.print("}")
		p.addNilSourceMapping()
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
	p.print(n.Data)
	invalidTSXAttributes := make([]Attribute, 0)
	for _, a := range n.Attr {
		if isInvalidTSXAttributeName(a.Key) {
			invalidTSXAttributes = append(invalidTSXAttributes, a)
			continue
		}
		p.print(" ")
		if a.Namespace != "" {
			p.print(a.Namespace)
			p.print(":")
		}
		switch a.Type {
		case astro.QuotedAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(a.Key)
			p.print("=")
			p.addSourceMapping(a.ValLoc)
			p.print(`"` + encodeDoubleQuote(a.Val) + `"`)
		case astro.EmptyAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(a.Key)
		case astro.ExpressionAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(a.Key)
			p.print("=")
			p.addSourceMapping(a.ValLoc)
			p.print(fmt.Sprintf(`{%s}`, a.Val))
		case astro.SpreadAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(a.Key)
			p.print("=")
			p.addSourceMapping(a.ValLoc)
			p.print(fmt.Sprintf(`{...%s}`, a.Val))
		case astro.ShorthandAttribute:
			withoutComments := removeComments(a.Key)
			if len(withoutComments) == 0 {
				return
			}
			p.addSourceMapping(a.KeyLoc)
			p.print(a.Key)
			p.print("=")
			p.addSourceMapping(a.KeyLoc)
			p.print(fmt.Sprintf(`{%s}`, a.Key))
		case astro.TemplateLiteralAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(a.Key)
			p.print("=")
			p.addSourceMapping(a.ValLoc)
			p.print(fmt.Sprintf("{`%s`}", escapeText(a.Val)))
		}
	}
	for i, a := range invalidTSXAttributes {
		if i == 0 {
			p.print(" {...{")
		} else {
			p.print(",")
		}
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
			eqStart := a.KeyLoc.Start + strings.IndexRune(p.sourcetext[a.KeyLoc.Start:], '=')
			p.addSourceMapping(loc.Loc{Start: eqStart})
			p.print(`:`)
			p.addSourceMapping(loc.Loc{Start: eqStart + 1})
			p.print(`"` + encodeDoubleQuote(a.Val) + `"`)
			p.addSourceMapping(loc.Loc{Start: eqStart + 1 + len(a.Val) + 2})
		case astro.EmptyAttribute:
			p.print(a.Key)
			p.print(`": true`)
		case astro.ExpressionAttribute:
			p.print(a.Key)
			p.print(`":`)
			p.addSourceMapping(a.ValLoc)
			p.print(fmt.Sprintf(`(%s)`, a.Val))
		case astro.SpreadAttribute:
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
		p.print("/>")
		return
	}
	p.print(">")

	// Render any child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderTsx(p, c)
	}
	// if len(n.Loc) == 2 {
	// 	p.addSourceMapping(n.Loc[1])
	// } else {
	// 	p.addSourceMapping(n.Loc[0])
	// }
	p.print(fmt.Sprintf(`</%s>`, n.Data))
}
