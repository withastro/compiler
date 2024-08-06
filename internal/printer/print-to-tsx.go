package printer

import (
	"fmt"
	"strings"
	"unicode"

	. "github.com/withastro/compiler/internal"
	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/handler"
	"github.com/withastro/compiler/internal/helpers"
	"github.com/withastro/compiler/internal/js_scanner"
	"github.com/withastro/compiler/internal/loc"
	"github.com/withastro/compiler/internal/sourcemap"
	"github.com/withastro/compiler/internal/transform"
	"golang.org/x/net/html/atom"
)

func getTSXPrefix() string {
	return "/* @jsxImportSource astro */\n\n"
}

type TSXOptions struct {
	IncludeScripts bool
	IncludeStyles  bool
}

func PrintToTSX(sourcetext string, n *Node, opts TSXOptions, transformOpts transform.TransformOptions, h *handler.Handler) PrintResult {
	p := &printer{
		sourcetext: sourcetext,
		opts:       transformOpts,
		builder:    sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(sourcetext, len(strings.Split(sourcetext, "\n")))),
	}
	p.print(getTSXPrefix())
	renderTsx(p, n, &opts)

	return PrintResult{
		Output:         p.output,
		SourceMapChunk: p.builder.GenerateChunk(p.output),
		TSXRanges:      finalizeRanges(string(p.output), p.ranges),
	}
}

func finalizeRanges(content string, ranges TSXRanges) TSXRanges {
	chunkBuilder := sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(content, len(strings.Split(content, "\n"))))

	return TSXRanges{
		Frontmatter: loc.TSXRange{
			Start: chunkBuilder.OffsetAt(loc.Loc{Start: ranges.Frontmatter.Start}),
			End:   chunkBuilder.OffsetAt(loc.Loc{Start: ranges.Frontmatter.End}),
		},
		Body: loc.TSXRange{
			Start: chunkBuilder.OffsetAt(loc.Loc{Start: ranges.Body.Start}),
			End:   chunkBuilder.OffsetAt(loc.Loc{Start: ranges.Body.End}),
		},
		// Scripts and styles are already using the proper positions
		Scripts: ranges.Scripts,
		Styles:  ranges.Styles,
	}
}

type TSXRanges struct {
	Frontmatter loc.TSXRange      `js:"frontmatter"`
	Body        loc.TSXRange      `js:"body"`
	Scripts     []TSXExtractedTag `js:"scripts"`
	Styles      []TSXExtractedTag `js:"styles"`
}

var htmlEvents = map[string]bool{
	"onabort":                   true,
	"onafterprint":              true,
	"onauxclick":                true,
	"onbeforematch":             true,
	"onbeforeprint":             true,
	"onbeforeunload":            true,
	"onblur":                    true,
	"oncancel":                  true,
	"oncanplay":                 true,
	"oncanplaythrough":          true,
	"onchange":                  true,
	"onclick":                   true,
	"onclose":                   true,
	"oncontextlost":             true,
	"oncontextmenu":             true,
	"oncontextrestored":         true,
	"oncopy":                    true,
	"oncuechange":               true,
	"oncut":                     true,
	"ondblclick":                true,
	"ondrag":                    true,
	"ondragend":                 true,
	"ondragenter":               true,
	"ondragleave":               true,
	"ondragover":                true,
	"ondragstart":               true,
	"ondrop":                    true,
	"ondurationchange":          true,
	"onemptied":                 true,
	"onended":                   true,
	"onerror":                   true,
	"onfocus":                   true,
	"onformdata":                true,
	"onhashchange":              true,
	"oninput":                   true,
	"oninvalid":                 true,
	"onkeydown":                 true,
	"onkeypress":                true,
	"onkeyup":                   true,
	"onlanguagechange":          true,
	"onload":                    true,
	"onloadeddata":              true,
	"onloadedmetadata":          true,
	"onloadstart":               true,
	"onmessage":                 true,
	"onmessageerror":            true,
	"onmousedown":               true,
	"onmouseenter":              true,
	"onmouseleave":              true,
	"onmousemove":               true,
	"onmouseout":                true,
	"onmouseover":               true,
	"onmouseup":                 true,
	"onoffline":                 true,
	"ononline":                  true,
	"onpagehide":                true,
	"onpageshow":                true,
	"onpaste":                   true,
	"onpause":                   true,
	"onplay":                    true,
	"onplaying":                 true,
	"onpopstate":                true,
	"onprogress":                true,
	"onratechange":              true,
	"onrejectionhandled":        true,
	"onreset":                   true,
	"onresize":                  true,
	"onscroll":                  true,
	"onscrollend":               true,
	"onsecuritypolicyviolation": true,
	"onseeked":                  true,
	"onseeking":                 true,
	"onselect":                  true,
	"onslotchange":              true,
	"onstalled":                 true,
	"onstorage":                 true,
	"onsubmit":                  true,
	"onsuspend":                 true,
	"ontimeupdate":              true,
	"ontoggle":                  true,
	"onunhandledrejection":      true,
	"onunload":                  true,
	"onvolumechange":            true,
	"onwaiting":                 true,
	"onwheel":                   true,
}

func getStyleLangFromAttrs(attrs []astro.Attribute) string {
	if len(attrs) == 0 {
		return "css"
	}

	for _, attr := range attrs {
		if attr.Key == "lang" {
			if attr.Type == astro.QuotedAttribute {
				return strings.TrimSpace(strings.ToLower(attr.Val))
			} else {
				// If the lang attribute exists, but is not quoted, we can't tell what's inside of it
				// So we'll just return "unknown" and let the downstream client decide what to do with it
				return "unknown"
			}
		}
	}

	return "css"
}

func getScriptTypeFromAttrs(attrs []astro.Attribute) string {
	if len(attrs) == 0 {
		return "processed-module"
	}

	for _, attr := range attrs {
		// If the script tag has `is:raw`, we can't tell what's inside of it
		// so the downstream client will decide what to do with it (e.g. ignore it, treat as inline, try to guess the type, etc.)
		if attr.Key == "is:raw" {
			return "raw"
		}

		if attr.Key == "type" {
			if attr.Type == astro.QuotedAttribute {
				normalizedType := strings.TrimSpace(strings.ToLower(attr.Val))
				// If the script tag has `type="module"`, it's not processed, but it's still a module
				if normalizedType == "module" {
					return "module"
				}

				if ScriptJSONMimeTypes[normalizedType] {
					return "json"
				}

				if ScriptMimeTypes[normalizedType] {
					return "inline"
				}
			}

			// If the type is not recognized, leave it as unknown
			return "unknown"
		}

	}

	// Otherwise, it's an inline script
	return "inline"
}

type TSXExtractedTag struct {
	Loc     loc.TSXRange `js:"position"`
	Type    string       `js:"type"`
	Content string       `js:"content"`
	Lang    string       `js:"lang"`
}

func isScript(p *astro.Node) bool {
	return p.DataAtom == atom.Script
}

func isStyle(p *astro.Node) bool {
	return p.DataAtom == atom.Style
}

// Has is:raw attribute
func isRawText(p *astro.Node) bool {
	for _, a := range p.Attr {
		if a.Key == "is:raw" {
			return true
		}
	}
	return false
}

var ScriptMimeTypes map[string]bool = map[string]bool{
	"module":                 true,
	"text/typescript":        true,
	"application/javascript": true,
	"text/partytown":         true,
	"application/node":       true,
}

var ScriptJSONMimeTypes map[string]bool = map[string]bool{
	"application/json":    true,
	"application/ld+json": true,
	"importmap":           true,
	"speculationrules":    true,
}

// This is not perfect (as in, you wouldn't use this to make a spec compliant parser), but it's good enough
// for the real world. Thankfully, JSX is also a bit more lax than JavaScript, so we can spare some work.
func isValidTSXAttribute(a Attribute) bool {
	if a.Type == SpreadAttribute {
		return true
	}

	for i, ch := range a.Key {
		if i == 0 && !isValidFirstRune(ch) {
			return false
		}

		// See https://tc39.es/ecma262/#prod-IdentifierName
		if i != 0 && !(isValidFirstRune(ch) ||
			unicode.In(ch, unicode.Mn, unicode.Mc, unicode.Nd, unicode.Pc)) &&
			// : is allowed inside TSX attributes, for namespaces purpose
			// See https://facebook.github.io/jsx/#prod-JSXNamespacedName
			// - is allowed inside TSX attributes, for custom attributes
			// See https://facebook.github.io/jsx/#prod-JSXIdentifier
			ch != ':' && ch != '-' {
			return false
		}
	}

	return true
}

// See https://tc39.es/ecma262/#prod-IdentifierStartChar
func isValidFirstRune(r rune) bool {
	return r == '$' || r == '_' || unicode.In(r,
		unicode.Lu,
		unicode.Ll,
		unicode.Lt,
		unicode.Lm,
		unicode.Lo,
		unicode.Nl,
	)
}

type TextType uint32

const (
	RawText TextType = iota
	Text
	ScriptText
	JsonScriptText
	UnknownScriptText
	StyleText
)

func getTextType(n *astro.Node) TextType {
	if script := n.Closest(isScript); script != nil {
		attr := astro.GetAttribute(script, "type")
		if attr == nil || ScriptMimeTypes[strings.ToLower(attr.Val)] {
			return ScriptText
		}

		// There's no difference between JSON and unknown script types in the result JSX at this time
		// however, we might want to add some special handling in the future, so we keep them separate
		if ScriptJSONMimeTypes[strings.ToLower(attr.Val)] {
			return JsonScriptText
		}

		return UnknownScriptText
	}
	if style := n.Closest(isStyle); style != nil {
		return StyleText
	}

	if n.Closest(isRawText) != nil {
		return RawText
	}

	return Text
}

func renderTsx(p *printer, n *Node, o *TSXOptions) {
	// Root of the document, print all children
	if n.Type == DocumentNode {
		source := []byte(p.sourcetext)
		props := js_scanner.GetPropsType(source)
		hasGetStaticPaths := js_scanner.HasGetStaticPaths(source)
		hasChildren := false
		startLen := len(p.output)
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			// This checks for the first node that comes *after* the frontmatter
			// to ensure that the statement is properly closed with a `;`.
			// Without this, TypeScript can get tripped up by the body of our file.
			if c.PrevSibling != nil && c.PrevSibling.Type == FrontmatterNode {
				buf := strings.TrimSpace(string(p.output))
				if len(buf)-len(getTSXPrefix()) > 1 {
					char := rune(buf[len(buf)-1:][0])
					// If the existing buffer ends with any character other than ;, we need to add a `;`
					if char != ';' {
						p.addNilSourceMapping()
						p.print("{};")
					}
				}
				// We always need to start the body with `<Fragment>`
				p.addNilSourceMapping()
				p.print("<Fragment>\n")

				// Update the start location of the body to the start of the first child
				startLen = len(p.output)

				hasChildren = true
			}
			if c.PrevSibling == nil && c.Type != FrontmatterNode {
				p.addNilSourceMapping()
				p.print("<Fragment>\n")

				startLen = len(p.output)

				hasChildren = true
			}
			renderTsx(p, c, o)
		}
		p.addSourceMapping(loc.Loc{Start: len(p.sourcetext)})
		p.print("\n")

		p.addNilSourceMapping()
		p.setTSXBodyRange(loc.TSXRange{
			Start: startLen,
			End:   len(p.output),
		})

		// Only close the body with `</Fragment>` if we printed a body
		if hasChildren {
			p.print("</Fragment>\n")
		}
		componentName := getTSXComponentName(p.opts.Filename)
		propsIdent := props.Ident
		paramsIdent := ""
		if hasGetStaticPaths {
			paramsIdent = "ASTRO__Get<ASTRO__InferredGetStaticPath, 'params'>"
			if propsIdent == "Record<string, any>" {
				propsIdent = "ASTRO__MergeUnion<ASTRO__Get<ASTRO__InferredGetStaticPath, 'props'>>"
			}
		}

		p.print(fmt.Sprintf("export default function %s%s(_props: %s%s%s): any {}\n", componentName, props.Statement, propsIdent, props.Generics, ` & {
  [key in keyof (Omit<import("astro").AstroBuiltinProps, "server:defer"> & import("astro").AstroClientDirectives)]: never;
}`))
		if hasGetStaticPaths {
			p.printf(`type ASTRO__ArrayElement<ArrayType extends readonly unknown[]> = ArrayType extends readonly (infer ElementType)[] ? ElementType : never;
type ASTRO__Flattened<T> = T extends Array<infer U> ? ASTRO__Flattened<U> : T;
type ASTRO__InferredGetStaticPath = ASTRO__Flattened<ASTRO__ArrayElement<Awaited<ReturnType<typeof getStaticPaths>>>>;
type ASTRO__MergeUnion<T, K extends PropertyKey = T extends unknown ? keyof T : never> = T extends unknown ? T & { [P in Exclude<K, keyof T>]?: never } extends infer O ? { [P in keyof O]: O[P] } : never : never;
type ASTRO__Get<T, K> = T extends undefined ? undefined : K extends keyof T ? T[K] : never;%s`, "\n")
		}

		if propsIdent != "Record<string, any>" {
			p.printf(`/**
 * Astro global available in all contexts in .astro files
 *
 * [Astro documentation](https://docs.astro.build/reference/api-reference/#astro-global)
*/
declare const Astro: Readonly<import('astro').AstroGlobal<%s, typeof %s`, propsIdent, componentName)
			if paramsIdent != "" {
				p.printf(", %s", paramsIdent)
			}
			p.print(">>")
		}
		return
	}

	if n.Type == FrontmatterNode {
		p.addSourceMapping(loc.Loc{Start: 0})
		frontmatterStart := len(p.output)
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == TextNode {
				if len(c.Loc) > 0 {
					p.addSourceMapping(c.Loc[0])
				}
				p.printTextWithSourcemap(c.Data, c.Loc[0])
			} else {
				renderTsx(p, c, o)
			}
		}
		if n.FirstChild != nil {
			p.addSourceMapping(loc.Loc{Start: n.FirstChild.Loc[0].Start + len(n.FirstChild.Data)})
			p.print("")
			p.addSourceMapping(loc.Loc{Start: n.FirstChild.Loc[0].Start + len(n.FirstChild.Data) + 3})
			p.println("")
		}
		p.setTSXFrontmatterRange(loc.TSXRange{
			Start: frontmatterStart,
			End:   len(p.output),
		})
		return
	}

	switch n.Type {
	case TextNode:
		textType := getTextType(n)
		if textType == ScriptText {
			p.addNilSourceMapping()
			if o.IncludeScripts {
				p.print("\n{() => {")
				p.printTextWithSourcemap(n.Data, n.Loc[0])
				p.addNilSourceMapping()
				p.print("}}\n")
			}
			p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start + len(n.Data)})
		} else if textType == StyleText || textType == JsonScriptText || textType == RawText || textType == UnknownScriptText {
			p.addNilSourceMapping()
			if (textType == StyleText && o.IncludeStyles) || ((textType == JsonScriptText || textType == UnknownScriptText) && o.IncludeScripts) || textType == RawText {
				p.print("{`")
				p.printTextWithSourcemap(escapeText(n.Data), n.Loc[0])
				p.addNilSourceMapping()
				p.print("`}")
			}
			p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start + len(n.Data)})
		} else {
			p.printEscapedJSXTextWithSourcemap(n.Data, n.Loc[0])
		}
		return
	case ElementNode:
		// No-op.
	case CommentNode:
		// p.addSourceMapping(n.Loc[0])
		p.addNilSourceMapping()
		p.print("{/**")
		if !unicode.IsSpace(rune(n.Data[0])) {
			// always add a space after the opening comment
			p.print(" ")
		}
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
			renderTsx(p, c, o)
			if c.NextSibling == nil || c.NextSibling.Type == TextNode {
				p.addNilSourceMapping()
				p.print(`</Fragment>`)
			}
		}
		if len(n.Loc) > 1 {
			p.addSourceMapping(n.Loc[1])
		} else {
			p.addSourceMapping(n.Loc[0])
		}
		p.print("}")
		return
	}

	isImplicit := false
	for _, a := range n.Attr {
		if transform.IsImplicitNodeMarker(a) {
			isImplicit = true
			break
		}
	}

	if isImplicit {
		// Render any child nodes
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			renderTsx(p, c, o)
		}
		return
	}

	p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start - 1})
	p.print("<")
	p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start})
	p.print(n.Data)
	p.addSourceMapping(loc.Loc{Start: n.Loc[0].Start + len(n.Data)})

	invalidTSXAttributes := make([]Attribute, 0)
	endLoc := n.Loc[0].Start + len(n.Data)
	for _, a := range n.Attr {
		if !isValidTSXAttribute(a) {
			invalidTSXAttributes = append(invalidTSXAttributes, a)
			continue
		}
		offset := 1
		if a.Type != astro.ShorthandAttribute && a.Type != astro.SpreadAttribute {
			p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start - offset})
		}
		p.print(" ")
		eqStart := a.KeyLoc.Start + strings.IndexRune(p.sourcetext[a.KeyLoc.Start:], '=')
		if a.Type != astro.ShorthandAttribute && a.Type != astro.SpreadAttribute {
			p.addSourceMapping(a.KeyLoc)
		}
		if a.Namespace != "" {
			p.print(a.Namespace)
			p.print(":")
		}
		switch a.Type {
		case astro.QuotedAttribute:
			p.print(a.Key)
			p.addSourceMapping(loc.Loc{Start: eqStart})
			p.print("=")
			if len(a.Val) > 0 {
				p.addSourceMapping(loc.Loc{Start: a.ValLoc.Start - 1})
				p.print(`"`)
				p.printTextWithSourcemap(encodeDoubleQuote(a.Val), loc.Loc{Start: a.ValLoc.Start})
				p.addSourceMapping(loc.Loc{Start: a.ValLoc.Start + len(a.Val)})
				p.print(`"`)
				endLoc = a.ValLoc.Start + len(a.Val) + 1
			} else {
				p.addSourceMapping(loc.Loc{Start: a.ValLoc.Start - 1})
				p.print(`"`)
				p.addSourceMapping(loc.Loc{Start: a.ValLoc.Start})
				p.print(`"`)
				endLoc = a.ValLoc.Start
			}

			if _, ok := htmlEvents[a.Key]; ok {
				p.addTSXScript(p.builder.OffsetAt(a.ValLoc), p.builder.OffsetAt(loc.Loc{Start: endLoc}), a.Val, "event-attribute")
			}
			if a.Key == "style" {
				p.addTSXStyle(p.builder.OffsetAt(a.ValLoc), p.builder.OffsetAt(loc.Loc{Start: endLoc}), a.Val, "style-attribute", "css")
			}
		case astro.EmptyAttribute:
			p.print(a.Key)
			endLoc = a.KeyLoc.Start + len(a.Key)
		case astro.ExpressionAttribute:
			p.print(a.Key)
			p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start + len(a.Key)})
			p.print(`=`)
			p.addSourceMapping(loc.Loc{Start: eqStart + 1})
			p.print(`{`)
			p.printTextWithSourcemap(a.Val, loc.Loc{Start: eqStart + 2})
			p.addSourceMapping(loc.Loc{Start: eqStart + 2 + len(a.Val)})
			p.print(`}`)
			endLoc = eqStart + len(a.Val) + 2
		case astro.SpreadAttribute:
			p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start - 4})
			p.print("{")
			p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start - 3})
			p.print("...")
			p.printTextWithSourcemap(a.Key, a.KeyLoc)
			p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start + len(a.Key)})
			p.print("}")
			endLoc = a.KeyLoc.Start + len(a.Key) + 1
		case astro.ShorthandAttribute:
			withoutComments := helpers.RemoveComments(a.Key)
			if len(withoutComments) == 0 {
				return
			}
			p.addSourceMapping(a.KeyLoc)
			p.printf(a.Key)
			p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start - 1})
			p.printf("={")
			p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start})
			p.print(a.Key)
			p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start + len(a.Key)})
			p.print("}")
			endLoc = a.KeyLoc.Start + len(a.Key) + 1
		case astro.TemplateLiteralAttribute:
			p.print(a.Key)
			p.addSourceMapping(loc.Loc{Start: eqStart})
			p.print(`=`)
			p.addNilSourceMapping()
			p.print(`{`)
			p.addSourceMapping(loc.Loc{Start: a.ValLoc.Start - 1})
			p.print("`")
			p.printTextWithSourcemap(a.Val, a.ValLoc)
			p.addSourceMapping(loc.Loc{Start: a.ValLoc.Start + len(a.Val)})
			p.print("`")
			p.addNilSourceMapping()
			p.print(`}`)
			endLoc = a.ValLoc.Start + len(a.Val) + 1
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
		case astro.EmptyAttribute:
			p.print(a.Key)
			p.print(`"`)
			p.addNilSourceMapping()
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
			// noop
		case astro.ShorthandAttribute:
			withoutComments := helpers.RemoveComments(a.Key)
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
			p.print(fmt.Sprintf("`%s`", a.Val))
		}
		if i == len(invalidTSXAttributes)-1 {
			p.addNilSourceMapping()
			p.print("}}")
		}
	}
	if len(n.Attr) == 0 {
		endLoc = n.Loc[0].Start + len(n.Data) - 1
	}
	if endLoc == -1 {
		endLoc = 0
	}
	isSelfClosing := false
	hasLeadingSpace := false
	tmpLoc := endLoc
	leadingSpaceLoc := endLoc
	if len(p.sourcetext) > tmpLoc {
		for i := 0; i < len(p.sourcetext[tmpLoc:]); i++ {
			c := p.sourcetext[endLoc : endLoc+1][0]
			if c == '/' && len(p.sourcetext) > endLoc+1 && p.sourcetext[endLoc+1:][0] == '>' {
				isSelfClosing = true
				break
			} else if c == '>' {
				p.addSourceMapping(loc.Loc{Start: endLoc})
				endLoc++
				break
			} else if unicode.IsSpace(rune(c)) || (c == '\\' && p.sourcetext[endLoc+1:][0] == 'n') {
				hasLeadingSpace = true
				leadingSpaceLoc = endLoc
				endLoc++
			} else {
				endLoc++
			}
		}
	} else {
		endLoc++
	}

	if hasLeadingSpace {
		p.addSourceMapping(loc.Loc{Start: leadingSpaceLoc})
		p.print(" ")
		p.addSourceMapping(loc.Loc{Start: leadingSpaceLoc + 1})
	}

	if voidElements[n.Data] && n.FirstChild == nil {
		p.print("/>")
		return
	}
	if isSelfClosing && n.FirstChild == nil {
		p.addSourceMapping(loc.Loc{Start: endLoc})
		p.print("/>")
		return
	}
	p.print(">")

	startTagEndLoc := loc.Loc{Start: endLoc}

	// Render any child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderTsx(p, c, o)
		if len(c.Loc) > 1 {
			endLoc = c.Loc[1].Start + len(c.Data) + 1
		} else if len(c.Loc) == 1 {
			endLoc = c.Loc[0].Start + len(c.Data)
		}
	}

	if len(n.Loc) > 1 {
		endLoc = n.Loc[1].Start - 2
	} else if n.LastChild != nil && n.LastChild.Expression {
		if len(n.LastChild.Loc) > 1 {
			endLoc = n.LastChild.Loc[1].Start + 1
		}
	}

	if n.FirstChild != nil && (n.DataAtom == atom.Script || n.DataAtom == atom.Style) {
		tagContentEndLoc := loc.Loc{Start: endLoc}
		if endLoc > len(p.sourcetext) { // Sometimes, when tags are not closed properly, endLoc can be greater than the length of the source text, wonky stuff
			tagContentEndLoc.Start = len(p.sourcetext)
		}
		if n.DataAtom == atom.Script {
			p.addTSXScript(p.builder.OffsetAt(startTagEndLoc), p.builder.OffsetAt(tagContentEndLoc), n.FirstChild.Data, getScriptTypeFromAttrs(n.Attr))
		}
		if n.DataAtom == atom.Style {
			p.addTSXStyle(p.builder.OffsetAt(startTagEndLoc), p.builder.OffsetAt(tagContentEndLoc), n.FirstChild.Data, "tag", getStyleLangFromAttrs(n.Attr))
		}
	}

	// Special case because of trailing expression close in scripts
	if n.DataAtom == atom.Script {
		p.printf("</%s>", n.Data)
		return
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
	p.addSourceMapping(loc.Loc{Start: endLoc + 1})
}
