package printer

import (
	"fmt"
	"strings"

	astro "github.com/snowpackjs/astro/internal"
	"github.com/snowpackjs/astro/internal/loc"
	"github.com/snowpackjs/astro/internal/sourcemap"
	"github.com/snowpackjs/astro/internal/transform"
)

type PrintResult struct {
	Output         []byte
	SourceMapChunk sourcemap.Chunk
}

type printer struct {
	opts                       transform.TransformOptions
	output                     []byte
	builder                    sourcemap.ChunkBuilder
	hasFuncPrelude             bool
	hasInternalImports         bool
	needsCustomElementRegistry bool
}

var TEMPLATE_TAG = "$$render"
var CREATE_COMPONENT = "$$createComponent"
var RENDER_COMPONENT = "$$renderComponent"
var RENDER_SLOT = "$$renderSlot"
var ADD_ATTRIBUTE = "$$addAttribute"
var SPREAD_ATTRIBUTES = "$$spreadAttributes"
var DEFINE_STYLE_VARS = "$$defineStyleVars"
var DEFINE_SCRIPT_VARS = "$$defineScriptVars"
var RESULT = "$$result"
var SLOTS = "$$slots"
var BACKTICK = "`"

func (p *printer) print(text string) {
	p.output = append(p.output, text...)
}

func (p *printer) println(text string) {
	p.output = append(p.output, (text + "\n")...)
}

func (p *printer) printInternalImports(importSpecifier string) {
	if p.hasInternalImports {
		return
	}
	p.print(fmt.Sprintf("import {\n  %s\n} from \"%s\";\n", strings.Join([]string{
		"render as " + TEMPLATE_TAG,
		"createComponent as " + CREATE_COMPONENT,
		"renderComponent as " + RENDER_COMPONENT,
		"renderSlot as " + RENDER_SLOT,
		"addAttribute as " + ADD_ATTRIBUTE,
		"spreadAttributes as " + SPREAD_ATTRIBUTES,
		"defineStyleVars as " + DEFINE_STYLE_VARS,
		"defineScriptVars as " + DEFINE_SCRIPT_VARS,
	}, ",\n  "), importSpecifier))
	p.hasInternalImports = true
}

func (p *printer) printReturnOpen() {
	p.addNilSourceMapping()
	p.print("return ")
	p.printTemplateLiteralOpen()
}

func (p *printer) printReturnClose() {
	p.addNilSourceMapping()
	p.printTemplateLiteralClose()
	p.println(";")
}

func (p *printer) printTemplateLiteralOpen() {
	p.addNilSourceMapping()
	p.print(fmt.Sprintf("%s%s", TEMPLATE_TAG, BACKTICK))
}

func (p *printer) printTemplateLiteralClose() {
	p.addNilSourceMapping()
	p.print(BACKTICK)
}

func (p *printer) printFuncPrelude(componentName string) {
	if p.hasFuncPrelude {
		return
	}
	p.addNilSourceMapping()
	p.println("\n//@ts-ignore")
	p.println(fmt.Sprintf("const %s = %s(async (%s, $$props, %s) => {", componentName, CREATE_COMPONENT, RESULT, SLOTS))
	p.println(fmt.Sprintf("const Astro = %s.createAstro($$props);", RESULT))
	p.hasFuncPrelude = true
}

func (p *printer) printFuncSuffix(componentName string) {
	p.addNilSourceMapping()
	p.println("});")
	p.println(fmt.Sprintf("export default %s;", componentName))
}

func (p *printer) printAttributesToObject(n *astro.Node) {
	p.print("{")
	for i, a := range n.Attr {
		if i != 0 {
			p.print(",")
		}
		switch a.Type {
		case astro.QuotedAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(`"` + a.Key + `"`)
			p.print(":")
			p.addSourceMapping(a.ValLoc)
			p.print(`"` + a.Val + `"`)
		case astro.EmptyAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(`"` + a.Key + `"`)
			p.print(":")
			p.print("true")
		case astro.ExpressionAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(`"` + a.Key + `"`)
			p.print(":")
			p.addSourceMapping(a.ValLoc)
			p.print(`(` + a.Val + `)`)
		case astro.SpreadAttribute:
			p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start - 3})
			p.print(`...(` + strings.TrimSpace(a.Key) + `)`)
		case astro.ShorthandAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(`"` + strings.TrimSpace(a.Key) + `"`)
			p.print(":")
			p.addSourceMapping(a.KeyLoc)
			p.print(`(` + strings.TrimSpace(a.Key) + `)`)
		case astro.TemplateLiteralAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.print(`"` + strings.TrimSpace(a.Key) + `"`)
			p.print(":")
			p.print("`" + strings.TrimSpace(a.Key) + "`")
		}
	}
	p.print("}")
}

func (p *printer) printStyleOrScript(n *astro.Node) {
	p.addNilSourceMapping()
	p.print("{props:")
	p.printAttributesToObject(n)
	if n.FirstChild != nil && strings.TrimSpace(n.FirstChild.Data) != "" {
		p.print(",children:`")
		p.addSourceMapping(n.Loc[0])
		p.print(escapeText(strings.TrimSpace(n.FirstChild.Data)))
		p.addNilSourceMapping()
		p.print("`")
	}
	p.print("},\n")
}

func (p *printer) printAttribute(attr astro.Attribute) {
	if attr.Namespace != "" {
		p.print(attr.Namespace)
		p.print(":")
	}

	switch attr.Type {
	case astro.QuotedAttribute:
		p.print(" ")
		p.addSourceMapping(attr.KeyLoc)
		p.print(attr.Key)
		p.print("=")
		p.addSourceMapping(attr.ValLoc)
		p.print(`"` + attr.Val + `"`)
	case astro.EmptyAttribute:
		p.print(" ")
		p.addSourceMapping(attr.KeyLoc)
		p.print(attr.Key)
	case astro.ExpressionAttribute:
		p.print(fmt.Sprintf("${%s(", ADD_ATTRIBUTE))
		p.addSourceMapping(attr.ValLoc)
		p.print(strings.TrimSpace(attr.Val))
		p.addSourceMapping(attr.KeyLoc)
		p.print(`, "` + strings.TrimSpace(attr.Key) + `")}`)
	case astro.SpreadAttribute:
		p.print(fmt.Sprintf("${%s(", SPREAD_ATTRIBUTES))
		p.addSourceMapping(loc.Loc{Start: attr.KeyLoc.Start - 3})
		p.print(strings.TrimSpace(attr.Key))
		p.print(`, "` + strings.TrimSpace(attr.Key) + `")}`)
	case astro.ShorthandAttribute:
		p.print(fmt.Sprintf("${%s(", ADD_ATTRIBUTE))
		p.addSourceMapping(attr.KeyLoc)
		p.print(strings.TrimSpace(attr.Key))
		p.addSourceMapping(attr.KeyLoc)
		p.print(`, "` + strings.TrimSpace(attr.Key) + `")}`)
	case astro.TemplateLiteralAttribute:
		p.print(fmt.Sprintf("${%s(`", ADD_ATTRIBUTE))
		p.addSourceMapping(attr.ValLoc)
		p.print(strings.TrimSpace(attr.Val))
		p.addSourceMapping(attr.KeyLoc)
		p.print("`" + `, "` + strings.TrimSpace(attr.Key) + `")}`)
	}
}

func (p *printer) addSourceMapping(location loc.Loc) {
	p.builder.AddSourceMapping(location, p.output)
}

func (p *printer) addNilSourceMapping() {
	p.builder.AddSourceMapping(loc.Loc{Start: 0}, p.output)
}
