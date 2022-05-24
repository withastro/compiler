package printer

import (
	"fmt"
	"regexp"
	"strings"

	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/js_scanner"
	"github.com/withastro/compiler/internal/loc"
	"github.com/withastro/compiler/internal/sourcemap"
	"github.com/withastro/compiler/internal/transform"
	"golang.org/x/net/html/atom"
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
	hasTypedProps      bool
	hasInternalImports bool
	hasCSSImports      bool
}

var TEMPLATE_TAG = "$$render"
var CREATE_ASTRO = "$$createAstro"
var CREATE_COMPONENT = "$$createComponent"
var RENDER_COMPONENT = "$$renderComponent"
var UNESCAPE_HTML = "$$unescapeHTML"
var RENDER_SLOT = "$$renderSlot"
var ADD_ATTRIBUTE = "$$addAttribute"
var SPREAD_ATTRIBUTES = "$$spreadAttributes"
var DEFINE_STYLE_VARS = "$$defineStyleVars"
var DEFINE_SCRIPT_VARS = "$$defineScriptVars"
var CREATE_METADATA = "$$createMetadata"
var METADATA = "$$metadata"
var RESULT = "$$result"
var SLOTS = "$$slots"
var FRAGMENT = "Fragment"
var BACKTICK = "`"
var styleModuleSpecExp = regexp.MustCompile(`(\.css|\.pcss|\.postcss|\.sass|\.scss|\.styl|\.stylus|\.less)$`)

func (p *printer) print(text string) {
	p.output = append(p.output, text...)
}

func (p *printer) printf(format string, a ...interface{}) {
	p.output = append(p.output, []byte(fmt.Sprintf(format, a...))...)
}

func (p *printer) println(text string) {
	p.output = append(p.output, (text + "\n")...)
}

func (p *printer) printInternalImports(importSpecifier string) {
	if p.hasInternalImports {
		return
	}
	p.print("import {\n  ")
	p.print(FRAGMENT + ",\n  ")
	p.print("render as " + TEMPLATE_TAG + ",\n  ")
	p.print("createAstro as " + CREATE_ASTRO + ",\n  ")
	p.print("createComponent as " + CREATE_COMPONENT + ",\n  ")
	p.print("renderComponent as " + RENDER_COMPONENT + ",\n  ")
	p.print("unescapeHTML as " + UNESCAPE_HTML + ",\n  ")
	p.print("renderSlot as " + RENDER_SLOT + ",\n  ")
	p.print("addAttribute as " + ADD_ATTRIBUTE + ",\n  ")
	p.print("spreadAttributes as " + SPREAD_ATTRIBUTES + ",\n  ")
	p.print("defineStyleVars as " + DEFINE_STYLE_VARS + ",\n  ")
	p.print("defineScriptVars as " + DEFINE_SCRIPT_VARS + ",\n  ")
	p.print("createMetadata as " + CREATE_METADATA)
	p.print("\n} from \"")
	p.print(importSpecifier)
	p.print("\";\n")
	p.hasInternalImports = true
}

func (p *printer) printCSSImports(cssLen int) {
	if p.hasCSSImports {
		return
	}
	i := 0
	for i < cssLen {
		// import '/src/pages/index.astro?astro&type=style&index=0&lang.css';
		p.print(fmt.Sprintf("import \"%s?astro&type=style&index=%v&lang.css\";", p.opts.Filename, i))
		i++
	}
	p.print("\n")
	p.hasCSSImports = true
}

func (p *printer) printRenderHead() {
	p.addNilSourceMapping()
	p.print("<!--astro:head-->")
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

func (p *printer) printDefineVars(n *astro.Node) {
	// Only handle <script> or <style>
	if !(n.DataAtom == atom.Script || n.DataAtom == atom.Style) {
		return
	}
	for _, attr := range n.Attr {
		if attr.Key == "define:vars" {
			var value string
			var defineCall string

			if n.DataAtom == atom.Script {
				defineCall = DEFINE_SCRIPT_VARS
			} else if n.DataAtom == atom.Style {
				defineCall = DEFINE_STYLE_VARS
			}
			switch attr.Type {
			case astro.QuotedAttribute:
				value = `"` + attr.Val + `"`
			case astro.EmptyAttribute:
				value = attr.Key
			case astro.ExpressionAttribute:
				value = strings.TrimSpace(attr.Val)
			}
			p.addNilSourceMapping()
			p.print(fmt.Sprintf("${%s(", defineCall))
			p.addSourceMapping(attr.ValLoc)
			p.print(value)
			p.addNilSourceMapping()
			p.print(")}")
			return
		}
	}
}

func (p *printer) printFuncPrelude(opts transform.TransformOptions) {
	if p.hasFuncPrelude {
		return
	}
	componentName := getComponentName(opts.Pathname)
	p.addNilSourceMapping()
	p.println("\n//@ts-ignore")
	p.println(fmt.Sprintf("const %s = %s(async (%s, $$props, %s) => {", componentName, CREATE_COMPONENT, RESULT, SLOTS))
	p.println(fmt.Sprintf("const Astro = %s.createAstro($$Astro, $$props, %s);", RESULT, SLOTS))
	p.println(fmt.Sprintf("Astro.self = %s;", componentName))
	p.hasFuncPrelude = true
}

func (p *printer) printFuncSuffix(opts transform.TransformOptions) {
	componentName := getComponentName(opts.Pathname)
	p.addNilSourceMapping()
	p.println("});")
	p.println(fmt.Sprintf("export default %s;", componentName))
}

func (p *printer) printAttributesToObject(n *astro.Node) {
	lastAttributeSkipped := false
	p.print("{")
	for i, a := range n.Attr {
		if i != 0 && !lastAttributeSkipped {
			p.print(",")
		}
		if a.Key == "set:text" || a.Key == "set:html" || a.Key == "is:raw" {
			continue
		}
		lastAttributeSkipped = false
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
			if a.Val == "" {
				p.print(`(void 0)`)
			} else {
				p.print(`(` + a.Val + `)`)
			}
		case astro.SpreadAttribute:
			p.addSourceMapping(loc.Loc{Start: a.KeyLoc.Start - 3})
			p.print(`...(` + strings.TrimSpace(a.Key) + `)`)
		case astro.ShorthandAttribute:
			withoutComments := removeComments(a.Key)
			if len(withoutComments) == 0 {
				lastAttributeSkipped = true
				continue
			}
			p.addSourceMapping(a.KeyLoc)
			p.print(`"` + withoutComments + `"`)
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

func (p *printer) printStyleOrScript(opts RenderOptions, n *astro.Node) {
	// If this is using the StaticExtraction option, only define:vars
	// styles should be included in the STYLES array
	transformOpts := opts.opts
	if transformOpts.StaticExtraction {
		hasDefineVars := false
		for _, attr := range n.Attr {
			if attr.Key == "define:vars" {
				hasDefineVars = true
			}
		}

		if !hasDefineVars {
			return
		}
	}

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

func (p *printer) printAttribute(attr astro.Attribute, n *astro.Node) {
	if attr.Key == "define:vars" || attr.Key == "set:text" || attr.Key == "set:html" || attr.Key == "is:raw" {
		return
	}

	if attr.Namespace != "" || attr.Type == astro.QuotedAttribute || attr.Type == astro.EmptyAttribute {
		p.print(" ")
	}

	if attr.Namespace != "" {
		p.print(attr.Namespace)
		p.print(":")
	}

	switch attr.Type {
	case astro.QuotedAttribute:
		p.addSourceMapping(attr.KeyLoc)
		p.print(attr.Key)
		p.print("=")
		p.addSourceMapping(attr.ValLoc)
		p.print(`"` + encodeDoubleQuote(attr.Val) + `"`)
	case astro.EmptyAttribute:
		p.addSourceMapping(attr.KeyLoc)
		p.print(attr.Key)
	case astro.ExpressionAttribute:
		p.print(fmt.Sprintf("${%s(", ADD_ATTRIBUTE))
		p.addSourceMapping(attr.ValLoc)
		if strings.TrimSpace(attr.Val) == "" {
			p.print("(void 0)")
		} else {
			p.print(strings.TrimSpace(attr.Val))
		}
		p.addSourceMapping(attr.KeyLoc)
		p.print(`, "` + strings.TrimSpace(attr.Key) + `")}`)
	case astro.SpreadAttribute:
		injectClass := false
		for p := n.Parent; p != nil; p = p.Parent {
			if p.Parent == nil && len(p.Styles) != 0 {
				injectClass = true
				break
			}
		}
		if injectClass {
			for _, a := range n.Attr {
				if a.Key == "class" || a.Key == "class:list" {
					injectClass = false
					break
				}
			}
		}
		p.print(fmt.Sprintf("${%s(", SPREAD_ATTRIBUTES))
		p.addSourceMapping(loc.Loc{Start: attr.KeyLoc.Start - 3})
		p.print(strings.TrimSpace(attr.Key))
		if !injectClass {
			p.printf(`,"%s")}`, strings.TrimSpace(attr.Key))
		} else {
			p.printf(`,"%s",{"class":"astro-%s"})}`, strings.TrimSpace(attr.Key), p.opts.Scope)
		}
	case astro.ShorthandAttribute:
		withoutComments := removeComments(attr.Key)
		if len(withoutComments) == 0 {
			return
		}
		p.print(fmt.Sprintf("${%s(", ADD_ATTRIBUTE))
		p.addSourceMapping(attr.KeyLoc)
		p.print(strings.TrimSpace(attr.Key))
		p.addSourceMapping(attr.KeyLoc)
		p.print(`, "` + withoutComments + `")}`)
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

func (p *printer) printTopLevelAstro(opts transform.TransformOptions) {
	patharg := opts.Pathname
	if patharg == "" {
		patharg = "import.meta.url"
	} else {
		patharg = fmt.Sprintf("\"%s\"", patharg)
	}

	p.println(fmt.Sprintf("const $$Astro = %s(%s, '%s', '%s');\nconst Astro = $$Astro;", CREATE_ASTRO, patharg, p.opts.Site, p.opts.ProjectRoot))
}

func remove(slice []*astro.Node, node *astro.Node) []*astro.Node {
	var s int
	for i, n := range slice {
		if n == node {
			s = i
		}
	}
	return append(slice[:s], slice[s+1:]...)
}

func (p *printer) printComponentMetadata(doc *astro.Node, opts transform.TransformOptions, source []byte) {
	var specs []string
	var asrts []string
	var conlyspecs []string
	unfoundconly := make([]*astro.Node, len(doc.ClientOnlyComponents))
	copy(unfoundconly, doc.ClientOnlyComponents)

	modCount := 1
	loc, statement := js_scanner.NextImportStatement(source, 0)
	for loc != -1 {
		isClientOnlyImport := false
	component_loop:
		for _, n := range doc.ClientOnlyComponents {
			for _, imported := range statement.Imports {
				if imported.ExportName == "*" {
					prefix := fmt.Sprintf("%s.", imported.LocalName)

					if strings.HasPrefix(n.Data, prefix) {
						exportParts := strings.Split(n.Data[len(prefix):], ".")
						exportName := exportParts[0]
						// Inject metadata attributes to `client:only` Component
						pathAttr := astro.Attribute{
							Key:  "client:component-path",
							Val:  fmt.Sprintf(`$$metadata.resolvePath("%s")`, statement.Specifier),
							Type: astro.ExpressionAttribute,
						}
						n.Attr = append(n.Attr, pathAttr)
						conlyspecs = append(conlyspecs, statement.Specifier)

						exportAttr := astro.Attribute{
							Key:  "client:component-export",
							Val:  exportName,
							Type: astro.QuotedAttribute,
						}
						n.Attr = append(n.Attr, exportAttr)
						unfoundconly = remove(unfoundconly, n)

						isClientOnlyImport = true
						continue component_loop
					}
				} else if imported.LocalName == n.Data {
					// Inject metadata attributes to `client:only` Component
					pathAttr := astro.Attribute{
						Key:  "client:component-path",
						Val:  fmt.Sprintf(`$$metadata.resolvePath("%s")`, statement.Specifier),
						Type: astro.ExpressionAttribute,
					}
					n.Attr = append(n.Attr, pathAttr)
					conlyspecs = append(conlyspecs, statement.Specifier)
					unfoundconly = remove(unfoundconly, n)

					exportAttr := astro.Attribute{
						Key:  "client:component-export",
						Val:  imported.ExportName,
						Type: astro.QuotedAttribute,
					}
					n.Attr = append(n.Attr, exportAttr)

					isClientOnlyImport = true
					continue component_loop
				}
			}
			if isClientOnlyImport {
				continue component_loop
			}
		}
		if !isClientOnlyImport {
			assertions := ""
			if statement.Assertions != "" {
				assertions += " assert "
				assertions += statement.Assertions
			}

			isCssImport := false
			if len(statement.Imports) == 0 && styleModuleSpecExp.MatchString(statement.Specifier) {
				isCssImport = true
			}

			if !isCssImport {
				p.print(fmt.Sprintf("\nimport * as $$module%v from '%s'%s;", modCount, statement.Specifier, assertions))
				specs = append(specs, statement.Specifier)
				asrts = append(asrts, statement.Assertions)
				modCount++
			}
		}
		loc, statement = js_scanner.NextImportStatement(source, loc)
	}
	if len(unfoundconly) > 0 {
		componentnames := ""
		for cindex, n := range unfoundconly {
			if cindex > 0 {
				componentnames += ", "
			}
			componentnames += n.Data
		}
		panic(fmt.Sprintf("Unable to find matching import statements for the client:only component: %s. A client:only component must match an import statement, either the default export or a named exported, and can't be derived from a variable in the frontmatter.", componentnames))
	}
	// If we added imports, add a line break.
	if modCount > 1 {
		p.print("\n")
	}

	// Call createMetadata
	patharg := opts.Pathname
	if patharg == "" {
		patharg = "import.meta.url"
	} else {
		patharg = fmt.Sprintf("\"%s\"", patharg)
	}
	p.print(fmt.Sprintf("\nexport const $$metadata = %s(%s, { ", CREATE_METADATA, patharg))

	// Add modules
	p.print("modules: [")
	for i := 1; i < modCount; i++ {
		if i > 1 {
			p.print(", ")
		}
		asrt := "{}"
		if asrts[i-1] != "" {
			asrt = asrts[i-1]
		}
		p.print(fmt.Sprintf("{ module: $$module%v, specifier: '%s', assert: %s }", i, specs[i-1], asrt))
	}
	p.print("]")

	// Hydrated Components
	p.print(", hydratedComponents: [")
	for i, node := range doc.HydratedComponents {
		if i > 0 {
			p.print(", ")
		}

		if node.CustomElement {
			p.print(fmt.Sprintf("'%s'", node.Data))
		} else {
			p.print(node.Data)
		}
	}
	// Client-Only Components
	p.print("], clientOnlyComponents: [")
	uniquespecs := make([]string, 0)
	i := 0
conly_loop:
	for _, spec := range conlyspecs {
		for _, uniq := range uniquespecs {
			if uniq == spec {
				continue conly_loop
			}
		}
		if i > 0 {
			p.print(", ")
		}
		p.print(fmt.Sprintf("'%s'", spec))
		i++
		uniquespecs = append(uniquespecs, spec)
	}
	p.print("], hydrationDirectives: new Set([")
	j := 0
	for directive := range doc.HydrationDirectives {
		if j > 0 {
			p.print(", ")
		}
		p.print(fmt.Sprintf("'%s'", directive))
		j++
	}
	// Hoisted scripts
	p.print("]), hoisted: [")
	for i, node := range doc.Scripts {
		if i > 0 {
			p.print(", ")
		}

		src := astro.GetAttribute(node, "src")
		if src != nil {
			p.print(fmt.Sprintf("{ type: 'remote', src: '%s' }", escapeSingleQuote(src.Val)))
		} else if node.FirstChild != nil {
			p.print(fmt.Sprintf("{ type: 'inline', value: `%s` }", escapeInterpolation(escapeBackticks(node.FirstChild.Data))))
		}
	}

	p.print("] });\n\n")
}
