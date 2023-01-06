package printer

import (
	"fmt"
	"regexp"
	"strings"

	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/handler"
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
	sourcetext         string
	opts               transform.TransformOptions
	output             []byte
	builder            sourcemap.ChunkBuilder
	handler            *handler.Handler
	hasFuncPrelude     bool
	hasInternalImports bool
	hasCSSImports      bool
}

var TEMPLATE_TAG = "$$render"
var CREATE_ASTRO = "$$createAstro"
var CREATE_COMPONENT = "$$createComponent"
var RENDER_COMPONENT = "$$renderComponent"
var RENDER_HEAD = "$$renderHead"
var MAYBE_RENDER_HEAD = "$$maybeRenderHead"
var UNESCAPE_HTML = "$$unescapeHTML"
var RENDER_SLOT = "$$renderSlot"
var MERGE_SLOTS = "$$mergeSlots"
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
	p.output = append(p.output, []byte(text)...)
}

func (p *printer) printf(format string, a ...interface{}) {
	p.print(fmt.Sprintf(format, a...))
}

func (p *printer) println(text string) {
	p.print(text + "\n")
}

func (p *printer) printTextWithSourcemap(text string, l loc.Loc) {
	start := l.Start
	lastPos := -1
	for pos, c := range text {
		diff := pos - lastPos
		p.addSourceMapping(loc.Loc{Start: start})
		p.print(string(c))
		start += diff
		lastPos = pos
	}
}

func (p *printer) printInternalImports(importSpecifier string, opts *RenderOptions) {
	if p.hasInternalImports {
		return
	}
	p.addNilSourceMapping()
	p.print("")
	p.print("import {\n  ")
	p.addNilSourceMapping()
	p.print(FRAGMENT + ",\n  ")
	p.addNilSourceMapping()
	p.print("render as " + TEMPLATE_TAG + ",\n  ")
	p.addNilSourceMapping()
	p.print("createAstro as " + CREATE_ASTRO + ",\n  ")
	p.addNilSourceMapping()
	p.print("createComponent as " + CREATE_COMPONENT + ",\n  ")
	p.addNilSourceMapping()
	p.print("renderComponent as " + RENDER_COMPONENT + ",\n  ")
	p.addNilSourceMapping()
	p.print("renderHead as " + RENDER_HEAD + ",\n  ")
	p.addNilSourceMapping()
	p.print("maybeRenderHead as " + MAYBE_RENDER_HEAD + ",\n  ")
	p.addNilSourceMapping()
	p.print("unescapeHTML as " + UNESCAPE_HTML + ",\n  ")
	p.addNilSourceMapping()
	p.print("renderSlot as " + RENDER_SLOT + ",\n  ")
	p.addNilSourceMapping()
	p.print("mergeSlots as " + MERGE_SLOTS + ",\n  ")
	p.addNilSourceMapping()
	p.print("addAttribute as " + ADD_ATTRIBUTE + ",\n  ")
	p.addNilSourceMapping()
	p.print("spreadAttributes as " + SPREAD_ATTRIBUTES + ",\n  ")
	p.addNilSourceMapping()
	p.print("defineStyleVars as " + DEFINE_STYLE_VARS + ",\n  ")
	p.addNilSourceMapping()
	p.print("defineScriptVars as " + DEFINE_SCRIPT_VARS + ",\n  ")

	// Only needed if using fallback `resolvePath` as it calls `$$metadata.resolvePath`
	if opts.opts.ResolvePath == nil {
		p.addNilSourceMapping()
		p.print("createMetadata as " + CREATE_METADATA)
	}
	p.addNilSourceMapping()
	p.print("\n} from \"")
	p.print(importSpecifier)
	p.print("\";\n")
	p.addNilSourceMapping()
	p.hasInternalImports = true
}

func (p *printer) printCSSImports(cssLen int) {
	if p.hasCSSImports {
		return
	}
	i := 0
	for i < cssLen {
		p.addNilSourceMapping()
		// import '/src/pages/index.astro?astro&type=style&index=0&lang.css';
		p.print(fmt.Sprintf("import \"%s?astro&type=style&index=%v&lang.css\";", p.opts.Filename, i))
		i++
	}
	p.print("\n")
	p.hasCSSImports = true
}

func (p *printer) printRenderHead() {
	p.addNilSourceMapping()
	p.print(fmt.Sprintf("${%s(%s)}", RENDER_HEAD, RESULT))
}

func (p *printer) printMaybeRenderHead() {
	p.addNilSourceMapping()
	p.print(fmt.Sprintf("${%s(%s)}", MAYBE_RENDER_HEAD, RESULT))
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

func isTypeModuleScript(n *astro.Node) bool {
	t := astro.GetAttribute(n, "type")
	if t != nil && t.Val == "module" {
		return true
	}
	return false
}

func (p *printer) printDefineVarsOpen(n *astro.Node) {
	// Only handle <script> or <style>
	if !(n.DataAtom == atom.Script || n.DataAtom == atom.Style) {
		return
	}
	if !transform.HasAttr(n, "define:vars") {
		return
	}
	if n.DataAtom == atom.Script {
		if !isTypeModuleScript(n) {
			p.print("(function(){")
		}
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
			case astro.ExpressionAttribute:
				value = strings.TrimSpace(attr.Val)
			}
			p.addNilSourceMapping()
			p.print(fmt.Sprintf("${%s(", defineCall))
			p.addSourceMapping(attr.ValLoc)
			p.printf(value)
			p.addNilSourceMapping()
			p.print(")}")
			return
		}
	}
}

func (p *printer) printDefineVarsClose(n *astro.Node) {
	// Only handle <script>
	if !(n.DataAtom == atom.Script) {
		return
	}
	if !transform.HasAttr(n, "define:vars") {
		return
	}
	if !isTypeModuleScript(n) {
		p.print("})();")
	}
}

func (p *printer) printFuncPrelude(opts transform.TransformOptions) {
	if p.hasFuncPrelude {
		return
	}
	componentName := getComponentName(opts.Pathname)
	p.addNilSourceMapping()
	p.println(fmt.Sprintf("const %s = %s(async (%s, $$props, %s) => {", componentName, CREATE_COMPONENT, RESULT, SLOTS))
	p.addNilSourceMapping()
	p.println(fmt.Sprintf("const Astro = %s.createAstro($$Astro, $$props, %s);", RESULT, SLOTS))
	p.addNilSourceMapping()
	p.println(fmt.Sprintf("Astro.self = %s;", componentName))
	p.hasFuncPrelude = true
}

func (p *printer) printFuncSuffix(opts transform.TransformOptions) {
	componentName := getComponentName(opts.Pathname)
	p.addNilSourceMapping()
	if len(opts.ModuleId) > 0 {
		escapedModuleId := strings.ReplaceAll(opts.ModuleId, "'", "\\'")
		p.println(fmt.Sprintf("}, '%s');", escapedModuleId))
	} else {
		p.println("});")
	}
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
			p.print(`"` + escapeDoubleQuote(a.Val) + `"`)
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
			withoutComments, _ := removeComments(a.Key)
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
		p.addNilSourceMapping()
		p.print(`="`)
		p.printTextWithSourcemap(encodeDoubleQuote(escapeInterpolation(escapeBackticks(attr.Val))), attr.ValLoc)
		p.addNilSourceMapping()
		p.print(`"`)
	case astro.EmptyAttribute:
		p.addSourceMapping(attr.KeyLoc)
		p.print(attr.Key)
	case astro.ExpressionAttribute:
		p.addNilSourceMapping()
		p.print(fmt.Sprintf("${%s(", ADD_ATTRIBUTE))
		if strings.TrimSpace(attr.Val) == "" {
			p.addNilSourceMapping()
			p.print("(void 0)")
		} else {
			p.printTextWithSourcemap(attr.Val, attr.ValLoc)
		}
		p.addNilSourceMapping()
		p.print(`, "`)
		p.addSourceMapping(attr.KeyLoc)
		p.print(attr.Key)
		p.addNilSourceMapping()
		p.print(`")}`)
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
		withoutComments, _ := removeComments(attr.Key)
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
	if location.Start < 0 {
		p.builder.AddSourceMapping(loc.Loc{Start: 0}, p.output)
	} else {
		p.builder.AddSourceMapping(location, p.output)
	}
}

// Reset sourcemap by pointing to last possible index
func (p *printer) addNilSourceMapping() {
	p.builder.AddSourceMapping(loc.Loc{Start: -1}, p.output)
}

func (p *printer) printTopLevelAstro(opts transform.TransformOptions) {
	patharg := opts.Pathname
	if patharg == "" {
		patharg = "import.meta.url"
	} else {
		patharg = fmt.Sprintf("\"%s\"", patharg)
	}

	p.println(fmt.Sprintf("const $$Astro = %s(%s);\nconst Astro = $$Astro;", CREATE_ASTRO, opts.InjectGlobals))
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
	unfoundconly := make([]*astro.Node, len(doc.ClientOnlyComponentNodes))
	copy(unfoundconly, doc.ClientOnlyComponentNodes)

	modCount := 1
	l, statement := js_scanner.NextImportStatement(source, 0)
	for l != -1 {
		isClientOnlyImport := false
	component_loop:
		for _, n := range doc.ClientOnlyComponentNodes {
			for _, imported := range statement.Imports {
				if imported.ExportName == "*" {
					prefix := fmt.Sprintf("%s.", imported.LocalName)

					if strings.HasPrefix(n.Data, prefix) {
						exportParts := strings.Split(n.Data[len(prefix):], ".")
						exportName := exportParts[0]
						attrTemplate := `"%s"`
						if opts.ResolvePath == nil {
							attrTemplate = `$$metadata.resolvePath("%s")`
						}
						// Inject metadata attributes to `client:only` Component
						pathAttr := astro.Attribute{
							Key:  "client:component-path",
							Val:  fmt.Sprintf(attrTemplate, transform.ResolveIdForMatch(statement.Specifier, &opts)),
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
					attrTemplate := `"%s"`
					if opts.ResolvePath == nil {
						attrTemplate = `$$metadata.resolvePath("%s")`
					}
					// Inject metadata attributes to `client:only` Component
					pathAttr := astro.Attribute{
						Key:  "client:component-path",
						Val:  fmt.Sprintf(attrTemplate, transform.ResolveIdForMatch(statement.Specifier, &opts)),
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
		if !isClientOnlyImport && opts.ResolvePath == nil {
			assertions := ""
			if statement.Assertions != "" {
				assertions += " assert "
				assertions += statement.Assertions
			}

			isCSSImport := false
			if len(statement.Imports) == 0 && styleModuleSpecExp.MatchString(statement.Specifier) {
				isCSSImport = true
			}

			if !isCSSImport && !statement.IsType {
				p.print(fmt.Sprintf("\nimport * as $$module%v from '%s'%s;", modCount, statement.Specifier, assertions))
				specs = append(specs, statement.Specifier)
				asrts = append(asrts, statement.Assertions)
				modCount++
			}
		}
		l, statement = js_scanner.NextImportStatement(source, l)
	}
	if len(unfoundconly) > 0 {
		for _, n := range unfoundconly {
			p.handler.AppendError(&loc.ErrorWithRange{
				Code:  loc.ERROR_FRAGMENT_SHORTHAND_ATTRS,
				Text:  "Unable to find matching import statement for client:only component",
				Hint:  "A client:only component must match an import statement, either the default export or a named exported, and can't be derived from a variable in the frontmatter.",
				Range: loc.Range{Loc: n.Loc[0], Len: len(n.Data)},
			})
		}
	}
	// If we added imports, add a line break.
	if modCount > 1 {
		p.print("\n")
	}

	// Only needed if using fallback `resolvePath` as it calls `$$metadata.resolvePath`
	if opts.ResolvePath != nil {
		return
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
	for i, node := range doc.HydratedComponentNodes {
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

		defineVars := astro.GetAttribute(node, "define:vars")
		src := astro.GetAttribute(node, "src")

		switch {
		case defineVars != nil:
			keys := js_scanner.GetObjectKeys([]byte(defineVars.Val))
			params := make([]byte, 0)
			for i, key := range keys {
				params = append(params, key...)
				if i < len(keys)-1 {
					params = append(params, ',')
				}
			}
			p.print(fmt.Sprintf("{ type: 'define:vars', value: `%s`, keys: '%s' }", escapeInterpolation(escapeBackticks(node.FirstChild.Data)), escapeSingleQuote(string(params))))
		case src != nil:
			p.print(fmt.Sprintf("{ type: 'external', src: '%s' }", escapeSingleQuote(src.Val)))
		case node.FirstChild != nil:
			p.print(fmt.Sprintf("{ type: 'inline', value: `%s` }", escapeInterpolation(escapeBackticks(node.FirstChild.Data))))
		}
	}

	p.print("] });\n\n")
}
