package printer

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/handler"
	"github.com/withastro/compiler/internal/helpers"
	"github.com/withastro/compiler/internal/js_scanner"
	"github.com/withastro/compiler/internal/loc"
	"github.com/withastro/compiler/internal/sourcemap"
	"github.com/withastro/compiler/internal/transform"
	"golang.org/x/net/html/atom"
)

type PrintResult struct {
	Output         []byte
	SourceMapChunk sourcemap.Chunk
	// Optional, used only for TSX output
	TSXRanges TSXRanges
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
	needsTransitionCSS bool

	// Optional, used only for TSX output
	ranges TSXRanges
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
var RENDER_TRANSITION = "$$renderTransition"
var CREATE_TRANSITION_SCOPE = "$$createTransitionScope"
var SPREAD_ATTRIBUTES = "$$spreadAttributes"
var DEFINE_STYLE_VARS = "$$defineStyleVars"
var DEFINE_SCRIPT_VARS = "$$defineScriptVars"
var CREATE_METADATA = "$$createMetadata"
var RENDER_SCRIPT = "$$renderScript"
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

func (p *printer) setTSXFrontmatterRange(frontmatterRange loc.TSXRange) {
	p.ranges.Frontmatter = frontmatterRange
}

func (p *printer) setTSXBodyRange(componentRange loc.TSXRange) {
	p.ranges.Body = componentRange
}

func (p *printer) addTSXScript(start int, end int, content string, scriptType string) {
	p.ranges.Scripts = append(p.ranges.Scripts, TSXExtractedTag{
		Loc: loc.TSXRange{
			Start: start,
			End:   end,
		},
		Content: content,
		Type:    scriptType,
	})
}

func (p *printer) addTSXStyle(start int, end int, content string, styleType string, styleLang string) {
	p.ranges.Styles = append(p.ranges.Styles, TSXExtractedTag{
		Loc: loc.TSXRange{
			Start: start,
			End:   end,
		},
		Content: content,
		Type:    styleType,
		Lang:    styleLang,
	})
}

func (p *printer) printTextWithSourcemap(text string, l loc.Loc) {
	start := l.Start
	skipNext := false
	for pos, c := range text {
		if skipNext {
			skipNext = false
			continue
		}

		// If we encounter a CRLF, map both characters to the same location
		if c == '\r' && len(text[pos:]) > 1 && text[pos+1] == '\n' {
			p.addSourceMapping(loc.Loc{Start: start})
			p.print("\r\n")
			start += 2
			skipNext = true
			continue
		}

		_, nextCharByteSize := utf8.DecodeRuneInString(text[pos:])
		p.addSourceMapping(loc.Loc{Start: start})
		p.print(string(c))
		start += nextCharByteSize
	}
}

func (p *printer) printEscapedJSXTextWithSourcemap(text string, l loc.Loc) {
	start := l.Start
	skipNext := false
	for pos, c := range text {
		if skipNext {
			skipNext = false
			continue
		}

		// If we encounter a CRLF, map both characters to the same location
		if c == '\r' && len(text[pos:]) > 1 && text[pos+1] == '\n' {
			p.addSourceMapping(loc.Loc{Start: start})
			p.print("\r\n")
			start += 2
			skipNext = true
			continue
		}

		// If we encounter characters invalid in JSX, escape them by putting them in a JS expression
		// No need to map, since it's just text. We also don't need to handle tags, since this is only for text nodes.
		if c == '>' || c == '}' {
			p.print("{`")
			p.print(string(c))
			p.print("`}")
			start++
			continue
		}

		_, nextCharByteSize := utf8.DecodeRuneInString(text[pos:])
		p.addSourceMapping(loc.Loc{Start: start})
		p.print(string(c))
		start += nextCharByteSize
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
	p.addNilSourceMapping()
	p.print("renderTransition as " + RENDER_TRANSITION + ",\n  ")
	p.addNilSourceMapping()
	p.print("createTransitionScope as " + CREATE_TRANSITION_SCOPE + ",\n  ")
	p.addNilSourceMapping()
	p.print("renderScript as " + RENDER_SCRIPT + ",\n  ")

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
	if p.needsTransitionCSS {
		p.addNilSourceMapping()
		p.print(fmt.Sprintf(`import "%s";`, p.opts.TransitionsAnimationURL))
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
	if n.DataAtom == atom.Style && transform.HasAttr(n, "is:inline") {
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

func (p *printer) printFuncPrelude(opts transform.TransformOptions, printAstroGlobal bool) {
	if p.hasFuncPrelude {
		return
	}
	componentName := getComponentName(opts.Filename)

	// Decide whether to print `async` if top-level await is used. Use a loose check for now.
	funcPrefix := ""
	if strings.Contains(p.sourcetext, "await") {
		funcPrefix = "async "
	}

	p.addNilSourceMapping()
	p.println(fmt.Sprintf("const %s = %s(%s(%s, $$props, %s) => {", componentName, CREATE_COMPONENT, funcPrefix, RESULT, SLOTS))
	if printAstroGlobal {
		p.addNilSourceMapping()
		p.println(fmt.Sprintf("const Astro = %s.createAstro($$Astro, $$props, %s);", RESULT, SLOTS))
		p.addNilSourceMapping()
		p.println(fmt.Sprintf("Astro.self = %s;", componentName))
	}
	p.hasFuncPrelude = true
}

func (p *printer) printFuncSuffix(opts transform.TransformOptions, n *astro.Node) {
	componentName := getComponentName(opts.Filename)
	p.addNilSourceMapping()
	filenameArg := "undefined"
	propagationArg := "undefined"
	if len(opts.Filename) > 0 {
		escapedFilename := strings.ReplaceAll(opts.Filename, "'", "\\'")
		filenameArg = fmt.Sprintf("'%s'", escapedFilename)
	}
	if n.Transition {
		propagationArg = "'self'"
	}
	p.println(fmt.Sprintf("}, %s, %s);", filenameArg, propagationArg))
	p.println(fmt.Sprintf("export default %s;", componentName))
}

var skippedAttributes = map[string]bool{
	"define:vars":        true,
	"set:text":           true,
	"set:html":           true,
	"is:raw":             true,
	"transition:animate": true,
	"transition:name":    true,
	"transition:persist": true,
}

var skippedAttributesToObject = map[string]bool{
	"set:text":           true,
	"set:html":           true,
	"is:raw":             true,
	"transition:animate": true,
	"transition:name":    true,
	"transition:persist": true,
}

func (p *printer) printAttributesToObject(n *astro.Node) {
	lastAttributeSkipped := false
	p.print("{")
	for i, a := range n.Attr {
		if i != 0 && !lastAttributeSkipped {
			p.print(",")
		}
		if _, ok := skippedAttributesToObject[a.Key]; ok {
			lastAttributeSkipped = true
			continue
		}
		if a.Namespace != "" {
			a.Key = fmt.Sprintf(`%s:%s`, a.Namespace, a.Key)
		}
		lastAttributeSkipped = false
		switch a.Type {
		case astro.QuotedAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.printf(`"%s"`, a.Key)
			p.print(":")
			p.addSourceMapping(a.ValLoc)
			p.print(`"` + escapeDoubleQuote(a.Val) + `"`)
		case astro.EmptyAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.printf(`"%s"`, a.Key)
			p.print(":")
			p.print("true")
		case astro.ExpressionAttribute:
			p.addSourceMapping(a.KeyLoc)
			p.printf(`"%s"`, a.Key)
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
			withoutComments := helpers.RemoveComments(a.Key)
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
			p.printf(`"%s"`, strings.TrimSpace(a.Key))
			p.print(":")
			p.print("`" + strings.TrimSpace(a.Val) + "`")
		}
	}
	p.print("}")
}

func (p *printer) printAttribute(attr astro.Attribute, n *astro.Node) {
	if _, ok := skippedAttributes[attr.Key]; ok {
		return
	}

	if attr.Namespace != "" || attr.Type == astro.QuotedAttribute || attr.Type == astro.EmptyAttribute {
		p.print(" ")
	}

	if attr.Namespace != "" {
		attr.Key = fmt.Sprintf("%s:%s", attr.Namespace, attr.Key)
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
			p.print(`)}`)
		} else {
			p.printf(`,undefined,{"class":"astro-%s"})}`, p.opts.Scope)
		}
	case astro.ShorthandAttribute:
		withoutComments := helpers.RemoveComments(attr.Key)
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
	p.println(fmt.Sprintf("const $$Astro = %s(%s);\nconst Astro = $$Astro;", CREATE_ASTRO, opts.AstroGlobalArgs))
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

func maybeConvertTransition(n *astro.Node) {
	if transform.HasAttr(n, transform.TRANSITION_ANIMATE) || transform.HasAttr(n, transform.TRANSITION_NAME) {
		animationExpr := convertAttributeValue(n, transform.TRANSITION_ANIMATE)
		transitionExpr := convertAttributeValue(n, transform.TRANSITION_NAME)

		n.Attr = append(n.Attr, astro.Attribute{
			Key:  "data-astro-transition-scope",
			Val:  fmt.Sprintf(`%s(%s, "%s", %s, %s)`, RENDER_TRANSITION, RESULT, n.TransitionScope, animationExpr, transitionExpr),
			Type: astro.ExpressionAttribute,
		})
	}
	if transform.HasAttr(n, transform.TRANSITION_PERSIST) {
		transitionPersistIndex := transform.AttrIndex(n, transform.TRANSITION_PERSIST)
		// If there no value, create a transition scope for this element
		if n.Attr[transitionPersistIndex].Val != "" {
			// Just rename the attribute
			n.Attr[transitionPersistIndex].Key = "data-astro-transition-persist"

		} else if transform.HasAttr(n, transform.TRANSITION_NAME) {
			transitionNameAttr := transform.GetAttr(n, transform.TRANSITION_NAME)
			n.Attr[transitionPersistIndex].Key = "data-astro-transition-persist"
			n.Attr[transitionPersistIndex].Val = transitionNameAttr.Val
			n.Attr[transitionPersistIndex].Type = transitionNameAttr.Type
		} else {
			n.Attr = append(n.Attr, astro.Attribute{
				Key:  "data-astro-transition-persist",
				Val:  fmt.Sprintf(`%s(%s, "%s")`, CREATE_TRANSITION_SCOPE, RESULT, n.TransitionScope),
				Type: astro.ExpressionAttribute,
			})
		}

		// Do a simple rename for `transition:persist-props`
		transitionPersistPropsIndex := transform.AttrIndex(n, transform.TRANSITION_PERSIST_PROPS)
		if transitionPersistPropsIndex != -1 {
			n.Attr[transitionPersistPropsIndex].Key = "data-astro-transition-persist-props"
		}
	}
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
				exportName, isUsed := js_scanner.ExtractComponentExportName(n.Data, imported)
				if isUsed {
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
	patharg := opts.Filename
	if patharg == "" {
		patharg = "import.meta.url"
	} else {
		escapedPatharg := strings.ReplaceAll(patharg, "'", "\\'")
		patharg = fmt.Sprintf("\"%s\"", escapedPatharg)
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
