package printer

import (
	"fmt"
	"strings"

	. "github.com/withastro/compiler/internal"
	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/sourcemap"
	"github.com/withastro/compiler/internal/transform"
)

func PrintToTSX(sourcetext string, n *Node, opts transform.TransformOptions) PrintResult {
	p := &printer{
		opts:    opts,
		builder: sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(sourcetext, len(strings.Split(sourcetext, "\n")))),
	}

	renderTsx(p, n, RenderOptions{
		isRoot: true,
		opts:   opts,
	})
	return PrintResult{
		Output:         p.output,
		SourceMapChunk: p.builder.GenerateChunk(p.output),
	}
}

func renderTsx(p *printer, n *Node, opts RenderOptions) {
	// Root of the document, print all children
	if n.Type == DocumentNode {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderTsx(p, c, RenderOptions{})
		}
		p.print("</Fragment>);\n}\n")
		return
	}

	if n.Type == FrontmatterNode {
		p.print("export default async (props) => {")
		p.hasFuncPrelude = true
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderTsx(p, c, RenderOptions{})
		}
		p.print("return (<Fragment>\n")
		return
	}

	if !p.hasFuncPrelude {
		p.print("export default async (props) => {")
		p.print("return (<Fragment>\n")
		p.hasFuncPrelude = true
	}

	switch n.Type {
	case TextNode:
		if strings.TrimSpace(n.Data) == "" {
			p.addSourceMapping(n.Loc[0])
			p.print(n.Data)
		} else {
			text := escapeBraces(n.Data)
			p.addSourceMapping(n.Loc[0])
			p.print(text)
		}
		return
	case ElementNode:
		// No-op.
	case CommentNode:
		p.addSourceMapping(n.Loc[0])
		p.print("{/* ")
		p.print(escapeBraces(n.Data))
		p.print(" */}")
		return
	default:
		return
	}

	if n.Expression {
		if n.FirstChild == nil {
			p.print("{(void 0)")
		} else if expressionOnlyHasCommentBlock(n) {
			// we do not print expressions that only contain comment blocks
			return
		} else {
			p.print("{")
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			p.addSourceMapping(c.Loc[0])
			if c.Type == TextNode {
				p.print(escapeBraces(c.Data))
				continue
			}
			if c.PrevSibling == nil || c.PrevSibling.Type == TextNode {
				p.print(`<Fragment>`)
			}
			renderTsx(p, c, RenderOptions{})
			if c.NextSibling == nil || c.NextSibling.Type == TextNode {
				p.print(`</Fragment>`)
			}
		}
		if len(n.Loc) >= 2 {
			p.addSourceMapping(n.Loc[1])
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
			renderTsx(p, c, RenderOptions{})
		}
		return
	}

	p.addSourceMapping(n.Loc[0])
	p.print("<")
	p.print(n.Data)
	for _, a := range n.Attr {
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
	if voidElements[n.Data] {
		p.print("/>")
		return
	}
	p.print(">")

	// Render any child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderTsx(p, c, RenderOptions{})
	}
	if len(n.Loc) == 2 {
		p.addSourceMapping(n.Loc[1])
	} else {
		p.addSourceMapping(n.Loc[0])
	}
	p.print(fmt.Sprintf(`</%s>`, n.Data))
}
