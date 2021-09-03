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
	opts               transform.TransformOptions
	output             []byte
	builder            sourcemap.ChunkBuilder
	hasFuncPrelude     bool
	hasInternalImports bool
}

var TEMPLATE_TAG = "$$render"
var CREATE_COMPONENT = "$$createComponent"
var RENDER_COMPONENT = "$$renderComponent"
var ADD_ATTRIBUTE = "$$addAttribute"
var SPREAD_ATTRIBUTES = "$$spreadAttributes"
var DEFINE_STYLE_VARS = "$$defineStyleVars"
var DEFINE_SCRIPT_VARS = "$$defineScriptVars"
var RESULT = "$$result"
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
	p.println("//@ts-ignore")
	p.println(fmt.Sprintf("const %s = %s(async (%s, $$props, $$slots) => {", componentName, CREATE_COMPONENT, RESULT))
	p.println(fmt.Sprintf("const Astro = %s.createAstro($$props);", RESULT))
	p.hasFuncPrelude = true
}

func (p *printer) printFuncSuffix(componentName string) {
	p.addNilSourceMapping()
	p.println("});")
	p.println(fmt.Sprintf("export default %s;", componentName))
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
