package printer

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"testing"

	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/handler"
	types "github.com/withastro/compiler/internal/t"
	"github.com/withastro/compiler/internal/test_utils"
	"github.com/withastro/compiler/internal/transform"
)

var INTERNAL_IMPORTS = fmt.Sprintf("import {\n  %s\n} from \"%s\";\n", strings.Join([]string{
	FRAGMENT,
	"render as " + TEMPLATE_TAG,
	"createAstro as " + CREATE_ASTRO,
	"createComponent as " + CREATE_COMPONENT,
	"renderComponent as " + RENDER_COMPONENT,
	"renderHead as " + RENDER_HEAD,
	"maybeRenderHead as " + MAYBE_RENDER_HEAD,
	"unescapeHTML as " + UNESCAPE_HTML,
	"renderSlot as " + RENDER_SLOT,
	"mergeSlots as " + MERGE_SLOTS,
	"addAttribute as " + ADD_ATTRIBUTE,
	"spreadAttributes as " + SPREAD_ATTRIBUTES,
	"defineStyleVars as " + DEFINE_STYLE_VARS,
	"defineScriptVars as " + DEFINE_SCRIPT_VARS,
	"renderTransition as " + RENDER_TRANSITION,
	"createTransitionScope as " + CREATE_TRANSITION_SCOPE,
	"renderScript as " + RENDER_SCRIPT,
	"createMetadata as " + CREATE_METADATA,
}, ",\n  "), "http://localhost:3000/")
var PRELUDE = fmt.Sprintf(`const $$Component = %s(async ($$result, $$props, %s) => {
const Astro = $$result.createAstro($$Astro, $$props, %s);
Astro.self = $$Component;%s`, CREATE_COMPONENT, SLOTS, SLOTS, "\n\n")
var RETURN = fmt.Sprintf("return %s%s", TEMPLATE_TAG, BACKTICK)
var SUFFIX = fmt.Sprintf("%s;", BACKTICK) + `
}, undefined, undefined);
export default $$Component;`
var SUFFIX_EXP_TRANSITIONS = fmt.Sprintf("%s;", BACKTICK) + `
}, undefined, 'self');
export default $$Component;`
var CREATE_ASTRO_CALL = "const $$Astro = $$createAstro('https://astro.build');\nconst Astro = $$Astro;"
var RENDER_HEAD_RESULT = "${$$renderHead($$result)}"

// SPECIAL TEST FIXTURES
var NON_WHITESPACE_CHARS = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[];:'\",.?")

func suffixWithFilename(filename string, transitions bool) string {
	propagationArg := "undefined"
	if transitions {
		propagationArg = `'self'`
	}
	return fmt.Sprintf("%s;", BACKTICK) + fmt.Sprintf(`
}, '%s', %s);
export default $$Component;`, filename, propagationArg)
}

type want struct {
	frontmatter    []string
	definedVars    []string
	getStaticPaths string
	code           string
	metadata
}

type metadata struct {
	hoisted              []string
	hydratedComponents   []string
	clientOnlyComponents []string
	modules              []string
	hydrationDirectives  []string
}

type testcase struct {
	name             string
	source           string
	only             bool
	transitions      bool
	transformOptions transform.TransformOptions
	filename         string
	want             want
}

type jsonTestcase struct {
	name   string
	source string
	want   []ASTNode
}

func TestPrinter(t *testing.T) {
	longRandomString := ""
	for i := 0; i < 4080; i++ {
		longRandomString += string(NON_WHITESPACE_CHARS[rand.Intn(len(NON_WHITESPACE_CHARS))])
	}

	tests := []testcase{
		{
			name:   "text only",
			source: `Foo`,
			want: want{
				code: `Foo`,
			},
		},
		{
			name:   "unusual line terminator I",
			source: `Pre-set & Time-limited \u2028holiday campaigns`,
			want: want{
				code: `Pre-set & Time-limited \\u2028holiday campaigns`,
			},
		},
		{
			name:   "unusual line terminator II",
			source: `Pre-set & Time-limited  holiday campaigns`,
			want: want{
				code: `Pre-set & Time-limited  holiday campaigns`,
			},
		},
		{
			name:   "basic (no frontmatter)",
			source: `<button>Click</button>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<button>Click</button>`,
			},
		},
		{
			name:   "basic renderHead",
			source: `<html><head><title>Ah</title></head></html>`,
			want: want{
				code: `<html><head><title>Ah</title>` + RENDER_HEAD_RESULT + `</head></html>`,
			},
		},
		{
			name:   "head inside slot",
			source: `<html><slot><head></head></slot></html>`,
			want: want{
				code: `<html>${$$renderSlot($$result,$$slots["default"],$$render` + BACKTICK + `<head>` + RENDER_HEAD_RESULT + `</head>` + BACKTICK + `)}</html>`,
			},
		},
		{
			name:   "head slot",
			source: `<html><head><slot /></html>`,
			want: want{
				code: `<html><head>${$$renderSlot($$result,$$slots["default"])}` + RENDER_HEAD_RESULT + `</head></html>`,
			},
		},
		{
			name:   "head slot II",
			source: `<html><head><slot /></head><body class="a"></body></html>`,
			want: want{
				code: `<html><head>${$$renderSlot($$result,$$slots["default"])}` + RENDER_HEAD_RESULT + `</head><body class="a"></body></html>`,
			},
		},
		{
			name:   "head slot III",
			source: `<html><head><slot name="baseHeadExtension"><meta property="test2" content="test2"/></slot></head>`,
			want: want{
				code: `<html><head>${$$renderSlot($$result,$$slots["baseHeadExtension"],$$render` + BACKTICK + `<meta property="test2" content="test2">` + BACKTICK + `)}` + RENDER_HEAD_RESULT + `</head></html>`,
			},
		},
		{
			name:   "ternary component",
			source: `{special ? <ChildDiv><p>Special</p></ChildDiv> : <p>Not special</p>}`,
			want: want{
				code: `${special ? $$render` + BACKTICK + `${$$renderComponent($$result,'ChildDiv',ChildDiv,{},{"default": () => $$render` + BACKTICK + `${$$maybeRenderHead($$result)}<p>Special</p>` + BACKTICK + `,})}` + BACKTICK + ` : $$render` + BACKTICK + `<p>Not special</p>` + BACKTICK + `}`,
			},
		},
		{
			name:   "ternary layout",
			source: `{toggleError ? <BaseLayout><h1>SITE: {Astro.site}</h1></BaseLayout> : <><h1>SITE: {Astro.site}</h1></>}`,
			want: want{
				code: `${toggleError ? $$render` + BACKTICK + `${$$renderComponent($$result,'BaseLayout',BaseLayout,{},{"default": () => $$render` + BACKTICK + `${$$maybeRenderHead($$result)}<h1>SITE: ${Astro.site}</h1>` + BACKTICK + `,})}` + BACKTICK + ` : $$render` + BACKTICK + `${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `<h1>SITE: ${Astro.site}</h1>` + BACKTICK + `,})}` + BACKTICK + `}`,
			},
		},
		{
			name:   "orphan slot",
			source: `<slot />`,
			want: want{
				code: `${$$renderSlot($$result,$$slots["default"])}`,
			},
		},
		{
			name:   "conditional slot",
			source: `<Component>{value && <div slot="test">foo</div>}</Component>`,
			want: want{
				code: "${$$renderComponent($$result,'Component',Component,{},{\"test\": () => $$render`${value && $$render`${$$maybeRenderHead($$result)}<div>foo</div>`}`,})}",
			},
		},
		{
			name:   "ternary slot",
			source: `<Component>{Math.random() > 0.5 ? <div slot="a">A</div> : <div slot="b">B</div>}</Component>`,
			want: want{
				code: "${$$renderComponent($$result,'Component',Component,{},$$mergeSlots({},Math.random() > 0.5 ? {\"a\": () => $$render`${$$maybeRenderHead($$result)}<div>A</div>`} : {\"b\": () => $$render`<div>B</div>`}))}",
			},
		},
		{
			name:   "function expression slots I",
			source: "<Component>\n{() => { switch (value) {\ncase 'a': return <div slot=\"a\">A</div>\ncase 'b': return <div slot=\"b\">B</div>\ncase 'c': return <div slot=\"c\">C</div>\n}\n}}\n</Component>",
			want: want{
				code: "${$$renderComponent($$result,'Component',Component,{},$$mergeSlots({},() => { switch (value) {\ncase 'a': return {\"a\": () => $$render`${$$maybeRenderHead($$result)}<div>A</div>`}\ncase 'b': return {\"b\": () => $$render`<div>B</div>`}\ncase 'c': return {\"c\": () => $$render`<div>C</div>`}}\n}))}",
			},
		},
		{
			name: "function expression slots II (#959)",
			source: `<Layout title="Welcome to Astro.">
	<main>
		<Layout title="switch bug">
			{components.map((component, i) => {
				switch(component) {
					case "Hero":
						return <div>Hero</div>
					case "Component2":
						return <div>Component2</div>
				}
			})}
		</Layout>
	</main>
</Layout>`,
			want: want{
				code: `${$$renderComponent($$result,'Layout',Layout,{"title":"Welcome to Astro."},{"default": () => $$render` + BACKTICK + `
	${$$maybeRenderHead($$result)}<main>
		${$$renderComponent($$result,'Layout',Layout,{"title":"switch bug"},{"default": () => $$render` + BACKTICK + `${components.map((component, i) => {
				switch(component) {
					case "Hero":
						return $$render` + BACKTICK + `<div>Hero</div>` + BACKTICK + `
					case "Component2":
						return $$render` + BACKTICK + `<div>Component2</div>` + BACKTICK + `
				}
			})}` + BACKTICK + `,})}
	</main>
` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "expression slot",
			source: `<Component>{true && <div slot="a">A</div>}{false && <div slot="b">B</div>}</Component>`,
			want: want{
				code: "${$$renderComponent($$result,'Component',Component,{},{\"a\": () => $$render`${true && $$render`${$$maybeRenderHead($$result)}<div>A</div>`}`,\"b\": () => $$render`${false && $$render`<div>B</div>`}`,})}",
			},
		},
		{
			name:   "preserve is:inline slot",
			source: `<slot is:inline />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<slot></slot>`,
			},
		},
		{
			name:   "preserve is:inline slot II",
			source: `<slot name="test" is:inline />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<slot name="test"></slot>`,
			},
		},
		{
			name:   "slot with fallback",
			source: `<body><slot><p>Hello world!</p></slot><body>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<body>${$$renderSlot($$result,$$slots["default"],$$render` + BACKTICK + `<p>Hello world!</p>` + BACKTICK + `)}</body>`,
			},
		},
		{
			name:   "slot with fallback II",
			source: `<slot name="test"><p>Hello world!</p></slot>`,
			want: want{
				code: `${$$renderSlot($$result,$$slots["test"],$$render` + BACKTICK + `${$$maybeRenderHead($$result)}<p>Hello world!</p>` + BACKTICK + `)}`,
			},
		},
		{
			name:   "slot with fallback III",
			source: `<div><slot name="test"><p>Fallback</p></slot></div>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div>${$$renderSlot($$result,$$slots["test"],$$render` + BACKTICK + `<p>Fallback</p>` + BACKTICK + `)}</div>`,
			},
		},
		{
			name: "Preserve slot whitespace",
			source: `<Component>
  <p>Paragraph 1</p>
  <p>Paragraph 2</p>
</Component>`,
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + BACKTICK + `
  ${$$maybeRenderHead($$result)}<p>Paragraph 1</p>
  <p>Paragraph 2</p>` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "text only",
			source: "Hello!",
			want: want{
				code: "Hello!",
			},
		},
		{
			name:   "custom-element",
			source: "{show && <client-only-element></client-only-element>}",
			want: want{
				code: "${show && $$render`${$$renderComponent($$result,'client-only-element','client-only-element',{})}`}",
			},
		},
		{
			name:   "attribute with template literal",
			source: "<a :href=\"`/home`\">Home</a>",
			want: want{
				code: "${$$maybeRenderHead($$result)}<a :href=\"\\`/home\\`\">Home</a>",
			},
		},
		{
			name:   "attribute with template literal interpolation",
			source: "<a :href=\"`/${url}`\">Home</a>",
			want: want{
				code: "${$$maybeRenderHead($$result)}<a :href=\"\\`/\\${url}\\`\">Home</a>",
			},
		},
		{
			name: "basic (frontmatter)",
			source: `---
const href = '/about';
---
<a href={href}>About</a>`,
			want: want{
				frontmatter: []string{"", "const href = '/about';"},
				code:        `${$$maybeRenderHead($$result)}<a${` + ADD_ATTRIBUTE + `(href, "href")}>About</a>`,
			},
		},
		{
			name: "getStaticPaths (basic)",
			source: `---
export const getStaticPaths = async () => {
	return { paths: [] }
}
---
<div></div>`,
			want: want{
				getStaticPaths: `export const getStaticPaths = async () => {
	return { paths: [] }
}`,
				code: `${$$maybeRenderHead($$result)}<div></div>`,
			},
		},
		{
			name: "getStaticPaths (hoisted)",
			source: `---
const a = 0;
export const getStaticPaths = async () => {
	return { paths: [] }
}
---
<div></div>`,
			want: want{
				frontmatter: []string{"", `const a = 0;`},
				getStaticPaths: `export const getStaticPaths = async () => {
	return { paths: [] }
}`,
				code: `${$$maybeRenderHead($$result)}<div></div>`,
			},
		},
		{
			name: "getStaticPaths (hoisted II)",
			source: `---
const a = 0;
export async function getStaticPaths() {
	return { paths: [] }
}
const b = 0;
---
<div></div>`,
			want: want{
				frontmatter: []string{"", `const a = 0;

const b = 0;`},
				getStaticPaths: `export async function getStaticPaths() {
	return { paths: [] }
}`,
				code: `${$$maybeRenderHead($$result)}<div></div>`,
			},
		},
		{
			name: "export member does not panic",
			source: `---
mod.export();
---
<div />`,
			want: want{
				frontmatter: []string{``, `mod.export();`},
				code:        `${$$maybeRenderHead($$result)}<div></div>`,
			},
		},
		{
			name: "export comments I",
			source: `---
// hmm
export const foo = 0
/*
*/
---`,
			want: want{
				frontmatter:    []string{"", "// hmm\n/*\n*/"},
				getStaticPaths: "export const foo = 0",
			},
		},
		{
			name: "export comments II",
			source: `---
// hmm
export const foo = 0;
/*
*/
---`,
			want: want{
				frontmatter:    []string{"", "// hmm\n/*\n*/"},
				getStaticPaths: "export const foo = 0;",
			},
		},
		{
			name: "import assertions",
			source: `---
import data from "test" assert { type: 'json' };
---
`,
			want: want{
				frontmatter: []string{
					`import data from "test" assert { type: 'json' };`,
				},
				metadata: metadata{modules: []string{`{ module: $$module1, specifier: 'test', assert: {type:'json'} }`}},
			},
		},
		{
			name: "import to identifier named assert",

			source: `---
import assert from 'test';
---`,
			want: want{
				frontmatter: []string{
					`import assert from 'test';`,
				},
				metadata: metadata{modules: []string{`{ module: $$module1, specifier: 'test', assert: {} }`}},
			},
		},
		{
			name:   "no expressions in math",
			source: `<p>Hello, world! This is a <em>buggy</em> formula: <span class="math math-inline"><span class="katex"><span class="katex-mathml"><math xmlns="http://www.w3.org/1998/Math/MathML"><semantics><mrow><mi>f</mi><mspace></mspace><mspace width="0.1111em"></mspace><mo lspace="0em" rspace="0.17em"></mo><mtext> ⁣</mtext><mo lspace="0em" rspace="0em">:</mo><mspace width="0.3333em"></mspace><mi>X</mi><mo>→</mo><msup><mi mathvariant="double-struck">R</mi><mrow><mn>2</mn><mi>x</mi></mrow></msup></mrow><annotation encoding="application/x-tex">f\colon X \to \mathbb R^{2x}</annotation></semantics></math></span><span class="katex-html" aria-hidden="true"><span class="base"><span class="strut" style="height:0.8889em;vertical-align:-0.1944em;"></span><span class="mord mathnormal" style="margin-right:0.10764em;">f</span><span class="mspace nobreak"></span><span class="mspace" style="margin-right:0.1111em;"></span><span class="mpunct"></span><span class="mspace" style="margin-right:-0.1667em;"></span><span class="mspace" style="margin-right:0.1667em;"></span><span class="mord"><span class="mrel">:</span></span><span class="mspace" style="margin-right:0.3333em;"></span><span class="mord mathnormal" style="margin-right:0.07847em;">X</span><span class="mspace" style="margin-right:0.2778em;"></span><span class="mrel">→</span><span class="mspace" style="margin-right:0.2778em;"></span></span><span class="base"><span class="strut" style="height:0.8141em;"></span><span class="mord"><span class="mord mathbb">R</span><span class="msupsub"><span class="vlist-t"><span class="vlist-r"><span class="vlist" style="height:0.8141em;"><span style="top:-3.063em;margin-right:0.05em;"><span class="pstrut" style="height:2.7em;"></span><span class="sizing reset-size6 size3 mtight"><span class="mord mtight"><span class="mord mtight">2</span><span class="mord mathnormal mtight">x</span></span></span></span></span></span></span></span></span></span></span></span></span></p>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<p>Hello, world! This is a <em>buggy</em> formula: <span class="math math-inline"><span class="katex"><span class="katex-mathml"><math xmlns="http://www.w3.org/1998/Math/MathML"><semantics><mrow><mi>f</mi><mspace></mspace><mspace width="0.1111em"></mspace><mo lspace="0em" rspace="0.17em"></mo><mtext> ⁣</mtext><mo lspace="0em" rspace="0em">:</mo><mspace width="0.3333em"></mspace><mi>X</mi><mo>→</mo><msup><mi mathvariant="double-struck">R</mi><mrow><mn>2</mn><mi>x</mi></mrow></msup></mrow><annotation encoding="application/x-tex">f\\colon X \\to \\mathbb R^{2x}</annotation></semantics></math></span><span class="katex-html" aria-hidden="true"><span class="base"><span class="strut" style="height:0.8889em;vertical-align:-0.1944em;"></span><span class="mord mathnormal" style="margin-right:0.10764em;">f</span><span class="mspace nobreak"></span><span class="mspace" style="margin-right:0.1111em;"></span><span class="mpunct"></span><span class="mspace" style="margin-right:-0.1667em;"></span><span class="mspace" style="margin-right:0.1667em;"></span><span class="mord"><span class="mrel">:</span></span><span class="mspace" style="margin-right:0.3333em;"></span><span class="mord mathnormal" style="margin-right:0.07847em;">X</span><span class="mspace" style="margin-right:0.2778em;"></span><span class="mrel">→</span><span class="mspace" style="margin-right:0.2778em;"></span></span><span class="base"><span class="strut" style="height:0.8141em;"></span><span class="mord"><span class="mord mathbb">R</span><span class="msupsub"><span class="vlist-t"><span class="vlist-r"><span class="vlist" style="height:0.8141em;"><span style="top:-3.063em;margin-right:0.05em;"><span class="pstrut" style="height:2.7em;"></span><span class="sizing reset-size6 size3 mtight"><span class="mord mtight"><span class="mord mtight">2</span><span class="mord mathnormal mtight">x</span></span></span></span></span></span></span></span></span></span></span></span></span></p>`,
			},
		},
		{
			name: "import order",
			source: `---
let testWord = "Test"
// comment
import data from "test";
---

<div>{data}</div>
`,
			want: want{
				frontmatter: []string{
					`import data from "test";`,
					"let testWord = \"Test\"\n// comment",
				},
				metadata: metadata{modules: []string{`{ module: $$module1, specifier: 'test', assert: {} }`}},
				code:     "${$$maybeRenderHead($$result)}<div>${data}</div>",
			},
		},
		{
			name: "type import",
			source: `---
import type data from "test"
---

<div>{data}</div>
`,
			want: want{
				frontmatter: []string{
					`import type data from "test"`,
				},
				code: "${$$maybeRenderHead($$result)}<div>${data}</div>",
			},
		},
		{
			name:   "no expressions in math",
			source: `<p>Hello, world! This is a <em>buggy</em> formula: <span class="math math-inline"><span class="katex"><span class="katex-mathml"><math xmlns="http://www.w3.org/1998/Math/MathML"><semantics><mrow><mi>f</mi><mspace></mspace><mspace width="0.1111em"></mspace><mo lspace="0em" rspace="0.17em"></mo><mtext> ⁣</mtext><mo lspace="0em" rspace="0em">:</mo><mspace width="0.3333em"></mspace><mi>X</mi><mo>→</mo><msup><mi mathvariant="double-struck">R</mi><mrow><mn>2</mn><mi>x</mi></mrow></msup></mrow><annotation encoding="application/x-tex">f\colon X \to \mathbb R^{2x}</annotation></semantics></math></span><span class="katex-html" aria-hidden="true"><span class="base"><span class="strut" style="height:0.8889em;vertical-align:-0.1944em;"></span><span class="mord mathnormal" style="margin-right:0.10764em;">f</span><span class="mspace nobreak"></span><span class="mspace" style="margin-right:0.1111em;"></span><span class="mpunct"></span><span class="mspace" style="margin-right:-0.1667em;"></span><span class="mspace" style="margin-right:0.1667em;"></span><span class="mord"><span class="mrel">:</span></span><span class="mspace" style="margin-right:0.3333em;"></span><span class="mord mathnormal" style="margin-right:0.07847em;">X</span><span class="mspace" style="margin-right:0.2778em;"></span><span class="mrel">→</span><span class="mspace" style="margin-right:0.2778em;"></span></span><span class="base"><span class="strut" style="height:0.8141em;"></span><span class="mord"><span class="mord mathbb">R</span><span class="msupsub"><span class="vlist-t"><span class="vlist-r"><span class="vlist" style="height:0.8141em;"><span style="top:-3.063em;margin-right:0.05em;"><span class="pstrut" style="height:2.7em;"></span><span class="sizing reset-size6 size3 mtight"><span class="mord mtight"><span class="mord mtight">2</span><span class="mord mathnormal mtight">x</span></span></span></span></span></span></span></span></span></span></span></span></span></p>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<p>Hello, world! This is a <em>buggy</em> formula: <span class="math math-inline"><span class="katex"><span class="katex-mathml"><math xmlns="http://www.w3.org/1998/Math/MathML"><semantics><mrow><mi>f</mi><mspace></mspace><mspace width="0.1111em"></mspace><mo lspace="0em" rspace="0.17em"></mo><mtext> ⁣</mtext><mo lspace="0em" rspace="0em">:</mo><mspace width="0.3333em"></mspace><mi>X</mi><mo>→</mo><msup><mi mathvariant="double-struck">R</mi><mrow><mn>2</mn><mi>x</mi></mrow></msup></mrow><annotation encoding="application/x-tex">f\\colon X \\to \\mathbb R^{2x}</annotation></semantics></math></span><span class="katex-html" aria-hidden="true"><span class="base"><span class="strut" style="height:0.8889em;vertical-align:-0.1944em;"></span><span class="mord mathnormal" style="margin-right:0.10764em;">f</span><span class="mspace nobreak"></span><span class="mspace" style="margin-right:0.1111em;"></span><span class="mpunct"></span><span class="mspace" style="margin-right:-0.1667em;"></span><span class="mspace" style="margin-right:0.1667em;"></span><span class="mord"><span class="mrel">:</span></span><span class="mspace" style="margin-right:0.3333em;"></span><span class="mord mathnormal" style="margin-right:0.07847em;">X</span><span class="mspace" style="margin-right:0.2778em;"></span><span class="mrel">→</span><span class="mspace" style="margin-right:0.2778em;"></span></span><span class="base"><span class="strut" style="height:0.8141em;"></span><span class="mord"><span class="mord mathbb">R</span><span class="msupsub"><span class="vlist-t"><span class="vlist-r"><span class="vlist" style="height:0.8141em;"><span style="top:-3.063em;margin-right:0.05em;"><span class="pstrut" style="height:2.7em;"></span><span class="sizing reset-size6 size3 mtight"><span class="mord mtight"><span class="mord mtight">2</span><span class="mord mathnormal mtight">x</span></span></span></span></span></span></span></span></span></span></span></span></span></p>`,
			},
		},

		{
			name: "css imports are not included in module metadata",
			source: `---
			import './styles.css';
			---
			`,
			want: want{
				frontmatter: []string{
					`import './styles.css';`,
				},
			},
		},
		{
			name:   "solidus in template literal expression",
			source: "<div value={`${attr ? `a/b` : \"c\"} awesome`} />",
			want: want{
				code: "${$$maybeRenderHead($$result)}<div${$$addAttribute(`${attr ? `a/b` : \"c\"} awesome`, \"value\")}></div>",
			},
		},
		{
			name:   "nested template literal expression",
			source: "<div value={`${attr ? `a/b ${`c`}` : \"d\"} awesome`} />",
			want: want{
				code: "${$$maybeRenderHead($$result)}<div${$$addAttribute(`${attr ? `a/b ${`c`}` : \"d\"} awesome`, \"value\")}></div>",
			},
		},
		{
			name:   "component in expression with its child expression before its child element",
			source: "{list.map(() => (<Component>{name}<link rel=\"stylesheet\" /></Component>))}",
			want: want{
				code: "${list.map(() => ($$render`${$$renderComponent($$result,'Component',Component,{},{\"default\": () => $$render`${name}<link rel=\"stylesheet\">`,})}`))}",
			},
		},
		{
			name: "expression returning multiple elements",
			source: `<Layout title="Welcome to Astro.">
	<main>
		<h1>Welcome to <span class="text-gradient">Astro</span></h1>
		{
			Object.entries(DUMMY_DATA).map(([dummyKey, dummyValue]) => {
				return (
					<p>
						onlyp {dummyKey}
					</p>
					<h2>
						onlyh2 {dummyKey}
					</h2>
					<div>
						<h2>div+h2 {dummyKey}</h2>
					</div>
					<p>
						<h2>p+h2 {dummyKey}</h2>
					</p>
				);
			})
		}
	</main>
</Layout>`,

			want: want{
				code: `${$$renderComponent($$result,'Layout',Layout,{"title":"Welcome to Astro."},{"default": () => $$render` + BACKTICK + `
	${$$maybeRenderHead($$result)}<main>
		<h1>Welcome to <span class="text-gradient">Astro</span></h1>
		${
			Object.entries(DUMMY_DATA).map(([dummyKey, dummyValue]) => {
				return (
					$$render` + BACKTICK + `<p>
						onlyp ${dummyKey}
					</p>
					<h2>
						onlyh2 ${dummyKey}
					</h2>
					<div>
						<h2>div+h2 ${dummyKey}</h2>
					</div>
					<p>
						</p><h2>p+h2 ${dummyKey}</h2>
					` + BACKTICK + `
				);
			})
		}
	</main>
` + BACKTICK + `,})}`,
			},
		},
		{
			name: "nested template literal expression",
			source: `<html lang="en">
<body>
{Object.keys(importedAuthors).map(author => <p><div>hello</div></p>)}
{Object.keys(importedAuthors).map(author => <p><div>{author}</div></p>)}
</body>
</html>`,
			want: want{
				code: `<html lang="en">
${$$maybeRenderHead($$result)}<body>
${Object.keys(importedAuthors).map(author => $$render` + BACKTICK + `<p></p><div>hello</div>` + BACKTICK + `)}
${Object.keys(importedAuthors).map(author => $$render` + BACKTICK + `<p></p><div>${author}</div>` + BACKTICK + `)}
</body>
</html>`,
			},
		},
		{
			name:   "complex nested template literal expression",
			source: "<div value={`${attr ? `a/b ${`c ${`d ${cool}`}`}` : \"d\"} ahhhh`} />",
			want: want{
				code: "${$$maybeRenderHead($$result)}<div${$$addAttribute(`${attr ? `a/b ${`c ${`d ${cool}`}`}` : \"d\"} ahhhh`, \"value\")}></div>",
			},
		},
		{
			name: "component",
			source: `---
import VueComponent from '../components/Vue.vue';
---
<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    <VueComponent />
  </body>
</html>`,
			want: want{
				frontmatter: []string{
					`import VueComponent from '../components/Vue.vue';`,
				},
				metadata: metadata{modules: []string{`{ module: $$module1, specifier: '../components/Vue.vue', assert: {} }`}},
				code: `<html>
  <head>
    <title>Hello world</title>
  ` + RENDER_HEAD_RESULT + `</head>
  <body>
    ${` + RENDER_COMPONENT + `($$result,'VueComponent',VueComponent,{})}
  </body></html>
  `,
			},
		},
		{
			name: "dot component",
			source: `---
import * as ns from '../components';
---
<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    <ns.Component />
  </body>
</html>`,
			want: want{
				frontmatter: []string{`import * as ns from '../components';`},
				metadata:    metadata{modules: []string{`{ module: $$module1, specifier: '../components', assert: {} }`}},
				code: `<html>
  <head>
    <title>Hello world</title>
  ` + RENDER_HEAD_RESULT + `</head>
  <body>
    ${` + RENDER_COMPONENT + `($$result,'ns.Component',ns.Component,{})}
  </body></html>
  `,
			},
		},
		{
			name:   "component with quoted attributes",
			source: `<Component is='"cool"' />`,
			want: want{
				code: `${` + RENDER_COMPONENT + `($$result,'Component',Component,{"is":"\"cool\""})}`,
			},
		},
		{
			name:   "slot with quoted attributes",
			source: `<Component><div slot='"name"' /></Component>`,
			want: want{
				code: `${` + RENDER_COMPONENT + `($$result,'Component',Component,{},{"\"name\"": () => $$render` + BACKTICK + `${$$maybeRenderHead($$result)}<div></div>` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "#955 ternary slot with text",
			source: `<Component>Hello{isLeaf ? <p>Leaf</p> : <p>Branch</p>}world</Component>`,
			want: want{
				code: `${` + RENDER_COMPONENT + `($$result,'Component',Component,{},{"default": () => $$render` + BACKTICK + `Hello${isLeaf ? $$render` + BACKTICK + `${$$maybeRenderHead($$result)}<p>Leaf</p>` + BACKTICK + ` : $$render` + BACKTICK + `<p>Branch</p>` + BACKTICK + `}world` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "#955 ternary slot with elements",
			source: `<Component><div>{isLeaf ? <p>Leaf</p> : <p>Branch</p>}</div></Component>`,
			want: want{
				code: `${` + RENDER_COMPONENT + `($$result,'Component',Component,{},{"default": () => $$render` + BACKTICK + `${$$maybeRenderHead($$result)}<div>${isLeaf ? $$render` + BACKTICK + `<p>Leaf</p>` + BACKTICK + ` : $$render` + BACKTICK + `<p>Branch</p>` + BACKTICK + `}</div>` + BACKTICK + `,})}`,
			},
		},
		{
			name: "noscript component",
			source: `
<html>
  <head></head>
  <body>
	<noscript>
		<Component />
	</noscript>
  </body>
</html>`,
			want: want{
				code: `<html>
  <head>` + RENDER_HEAD_RESULT + `</head>
  <body>
	<noscript>
		${` + RENDER_COMPONENT + `($$result,'Component',Component,{})}
	</noscript>
  </body></html>
  `,
			},
		},
		{
			name:   "noscript styles",
			source: `<noscript><style>div { color: red; }</style></noscript>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<noscript><style>div { color: red; }</style></noscript>`,
			},
		},
		{
			name:   "noscript deep styles",
			source: `<body><noscript><div><div><div><style>div { color: red; }</style></div></div></div></noscript></body>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<body><noscript><div><div><div><style>div { color: red; }</style></div></div></div></noscript></body>`,
			},
		},
		{
			name:   "noscript only",
			source: `<noscript><h1>Hello world</h1></noscript>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<noscript><h1>Hello world</h1></noscript>`,
			},
		},
		{
			name: "client:only component (default)",
			source: `---
import Component from '../components';
---
<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    <Component client:only />
  </body>
</html>`,
			want: want{
				frontmatter: []string{"import Component from '../components';"},
				metadata: metadata{
					hydrationDirectives:  []string{"only"},
					clientOnlyComponents: []string{"../components"},
				},
				// Specifically do NOT render any metadata here, we need to skip this import
				code: `<html>
  <head>
    <title>Hello world</title>
  ` + RENDER_HEAD_RESULT + `</head>
  <body>
    ${` + RENDER_COMPONENT + `($$result,'Component',null,{"client:only":true,"client:component-hydration":"only","client:component-path":($$metadata.resolvePath("../components")),"client:component-export":"default"})}
  </body></html>`,
			},
		},
		{
			name: "client:only component (named)",
			source: `---
import { Component } from '../components';
---
<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    <Component client:only />
  </body>
</html>`,
			want: want{
				frontmatter: []string{"import { Component } from '../components';"},
				metadata: metadata{
					hydrationDirectives:  []string{"only"},
					clientOnlyComponents: []string{"../components"},
				},
				// Specifically do NOT render any metadata here, we need to skip this import
				code: `<html>
  <head>
    <title>Hello world</title>
  ` + RENDER_HEAD_RESULT + `</head>
  <body>
    ${` + RENDER_COMPONENT + `($$result,'Component',null,{"client:only":true,"client:component-hydration":"only","client:component-path":($$metadata.resolvePath("../components")),"client:component-export":"Component"})}
  </body></html>`,
			},
		},
		{
			name: "client:only component (namespace)",
			source: `---
import * as components from '../components';
---
<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    <components.A client:only />
  </body>
</html>`,
			want: want{
				frontmatter: []string{"import * as components from '../components';"},
				metadata: metadata{
					hydrationDirectives:  []string{"only"},
					clientOnlyComponents: []string{"../components"},
				},
				// Specifically do NOT render any metadata here, we need to skip this import
				code: `<html>
  <head>
    <title>Hello world</title>
  ` + RENDER_HEAD_RESULT + `</head>
  <body>
    ${` + RENDER_COMPONENT + `($$result,'components.A',null,{"client:only":true,"client:component-hydration":"only","client:component-path":($$metadata.resolvePath("../components")),"client:component-export":"A"})}
  </body></html>`,
			},
		},
		{
			name: "client:only component (namespaced default)",
			source: `---
import defaultImport from '../components/ui-1';
---
<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
	<defaultImport.Counter1 client:only />
  </body>
</html>`,
			want: want{
				frontmatter: []string{"import defaultImport from '../components/ui-1';"},
				metadata: metadata{
					hydrationDirectives:  []string{"only"},
					clientOnlyComponents: []string{"../components/ui-1"},
				},
				// Specifically do NOT render any metadata here, we need to skip this import
				code: `<html>
  <head>
    <title>Hello world</title>
  ` + RENDER_HEAD_RESULT + `</head>
  <body>
	${` + RENDER_COMPONENT + `($$result,'defaultImport.Counter1',null,{"client:only":true,"client:component-hydration":"only","client:component-path":($$metadata.resolvePath("../components/ui-1")),"client:component-export":"default.Counter1"})}
  </body></html>`,
			},
		},
		{
			name: "client:only component (namespaced named)",
			source: `---
import { namedImport } from '../components/ui-2';
---
<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
	<namedImport.Counter2 client:only />
  </body>
</html>`,
			want: want{
				frontmatter: []string{"import { namedImport } from '../components/ui-2';"},
				metadata: metadata{
					hydrationDirectives:  []string{"only"},
					clientOnlyComponents: []string{"../components/ui-2"},
				},
				// Specifically do NOT render any metadata here, we need to skip this import
				code: `<html>
  <head>
    <title>Hello world</title>
  ` + RENDER_HEAD_RESULT + `</head>
  <body>
	${` + RENDER_COMPONENT + `($$result,'namedImport.Counter2',null,{"client:only":true,"client:component-hydration":"only","client:component-path":($$metadata.resolvePath("../components/ui-2")),"client:component-export":"namedImport.Counter2"})}
  </body></html>`,
			},
		},
		{
			name: "client:only component (multiple)",
			source: `---
import Component from '../components';
---
<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    <Component test="a" client:only />
	<Component test="b" client:only />
	<Component test="c" client:only />
  </body>
</html>`,
			want: want{
				frontmatter: []string{"import Component from '../components';"},
				metadata: metadata{
					hydrationDirectives:  []string{"only"},
					clientOnlyComponents: []string{"../components"},
				},
				// Specifically do NOT render any metadata here, we need to skip this import
				code: `<html>
  <head>
    <title>Hello world</title>
  ` + RENDER_HEAD_RESULT + `</head>
  <body>
    ${` + RENDER_COMPONENT + `($$result,'Component',null,{"test":"a","client:only":true,"client:component-hydration":"only","client:component-path":($$metadata.resolvePath("../components")),"client:component-export":"default"})}
	${` + RENDER_COMPONENT + `($$result,'Component',null,{"test":"b","client:only":true,"client:component-hydration":"only","client:component-path":($$metadata.resolvePath("../components")),"client:component-export":"default"})}
	${` + RENDER_COMPONENT + `($$result,'Component',null,{"test":"c","client:only":true,"client:component-hydration":"only","client:component-path":($$metadata.resolvePath("../components")),"client:component-export":"default"})}
  </body></html>`,
			},
		},
		{
			name:   "iframe",
			source: `<iframe src="something" />`,
			want: want{
				code: "${$$maybeRenderHead($$result)}<iframe src=\"something\"></iframe>",
			},
		},
		{
			name:   "conditional render",
			source: `<body>{false ? <div>#f</div> : <div>#t</div>}</body>`,
			want: want{
				code: "${$$maybeRenderHead($$result)}<body>${false ? $$render`<div>#f</div>` : $$render`<div>#t</div>`}</body>",
			},
		},
		{
			name:   "conditional noscript",
			source: `{mode === "production" && <noscript>Hello</noscript>}`,
			want: want{
				code: "${mode === \"production\" && $$render`${$$maybeRenderHead($$result)}<noscript>Hello</noscript>`}",
			},
		},
		{
			name:   "conditional iframe",
			source: `{bool && <iframe src="something">content</iframe>}`,
			want: want{
				code: "${bool && $$render`${$$maybeRenderHead($$result)}<iframe src=\"something\">content</iframe>`}",
			},
		},
		{
			name:   "simple ternary",
			source: `<body>{link ? <a href="/">{link}</a> : <div>no link</div>}</body>`,
			want: want{
				code: fmt.Sprintf(`${$$maybeRenderHead($$result)}<body>${link ? $$render%s<a href="/">${link}</a>%s : $$render%s<div>no link</div>%s}</body>`, BACKTICK, BACKTICK, BACKTICK, BACKTICK),
			},
		},
		{
			name: "map basic",
			source: `---
const items = [0, 1, 2];
---
<ul>
	{items.map(item => {
		return <li>{item}</li>;
	})}
</ul>`,
			want: want{
				frontmatter: []string{"", "const items = [0, 1, 2];"},
				code: fmt.Sprintf(`${$$maybeRenderHead($$result)}<ul>
	${items.map(item => {
		return $$render%s<li>${item}</li>%s;
	})}
</ul>`, BACKTICK, BACKTICK),
			},
		},
		{
			name:   "map without component",
			source: `<header><nav>{menu.map((item) => <a href={item.href}>{item.title}</a>)}</nav></header>`,
			want: want{
				code: fmt.Sprintf(`${$$maybeRenderHead($$result)}<header><nav>${menu.map((item) => $$render%s<a${$$addAttribute(item.href, "href")}>${item.title}</a>%s)}</nav></header>`, BACKTICK, BACKTICK),
			},
		},
		{
			name:   "map with component",
			source: `<header><nav>{menu.map((item) => <a href={item.href}>{item.title}</a>)}</nav><Hello/></header>`,
			want: want{
				code: fmt.Sprintf(`${$$maybeRenderHead($$result)}<header><nav>${menu.map((item) => $$render%s<a${$$addAttribute(item.href, "href")}>${item.title}</a>%s)}</nav>${$$renderComponent($$result,'Hello',Hello,{})}</header>`, BACKTICK, BACKTICK),
			},
		},
		{
			name: "map nested",
			source: `---
const groups = [[0, 1, 2], [3, 4, 5]];
---
<div>
	{groups.map(items => {
		return <ul>{
			items.map(item => {
				return <li>{item}</li>;
			})
		}</ul>
	})}
</div>`,
			want: want{
				frontmatter: []string{"", "const groups = [[0, 1, 2], [3, 4, 5]];"},
				code: fmt.Sprintf(`${$$maybeRenderHead($$result)}<div>
	${groups.map(items => {
		return %s<ul>${
			items.map(item => {
				return %s<li>${item}</li>%s;
			})
		}</ul>%s
	})}
</div>`, "$$render"+BACKTICK, "$$render"+BACKTICK, BACKTICK, BACKTICK),
			},
		},
		{
			name:   "backtick in HTML comment",
			source: "<body><!-- `npm install astro` --></body>",
			want: want{
				code: "${$$maybeRenderHead($$result)}<body><!-- \\`npm install astro\\` --></body>",
			},
		},
		{
			name:   "HTML comment in component inside expression I",
			source: "{(() => <Component><!--Hi--></Component>)}",
			want: want{
				code: "${(() => $$render`${$$renderComponent($$result,'Component',Component,{},{})}`)}",
			},
		},
		{
			name:   "HTML comment in component inside expression II",
			source: "{list.map(() => <Component><!--Hi--></Component>)}",
			want: want{
				code: "${list.map(() => $$render`${$$renderComponent($$result,'Component',Component,{},{})}`)}",
			},
		},
		{
			name:   "nested expressions",
			source: `<article>{(previous || next) && <aside>{previous && <div>Previous Article: <a rel="prev" href={new URL(previous.link, Astro.site).pathname}>{previous.text}</a></div>}{next && <div>Next Article: <a rel="next" href={new URL(next.link, Astro.site).pathname}>{next.text}</a></div>}</aside>}</article>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${(previous || next) && $$render` + BACKTICK + `<aside>${previous && $$render` + BACKTICK + `<div>Previous Article: <a rel="prev"${$$addAttribute(new URL(previous.link, Astro.site).pathname, "href")}>${previous.text}</a></div>` + BACKTICK + `}${next && $$render` + BACKTICK + `<div>Next Article: <a rel="next"${$$addAttribute(new URL(next.link, Astro.site).pathname, "href")}>${next.text}</a></div>` + BACKTICK + `}</aside>` + BACKTICK + `}</article>`,
			},
		},
		{
			name:   "nested expressions II",
			source: `<article>{(previous || next) && <aside>{previous && <div>Previous Article: <a rel="prev" href={new URL(previous.link, Astro.site).pathname}>{previous.text}</a></div>} {next && <div>Next Article: <a rel="next" href={new URL(next.link, Astro.site).pathname}>{next.text}</a></div>}</aside>}</article>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${(previous || next) && $$render` + BACKTICK + `<aside>${previous && $$render` + BACKTICK + `<div>Previous Article: <a rel="prev"${$$addAttribute(new URL(previous.link, Astro.site).pathname, "href")}>${previous.text}</a></div>` + BACKTICK + `} ${next && $$render` + BACKTICK + `<div>Next Article: <a rel="next"${$$addAttribute(new URL(next.link, Astro.site).pathname, "href")}>${next.text}</a></div>` + BACKTICK + `}</aside>` + BACKTICK + `}</article>`,
			},
		},
		{
			name:   "nested expressions III",
			source: `<div>{x.map((x) => x ? <div>{true ? <span>{x}</span> : null}</div> : <div>{false ? null : <span>{x}</span>}</div>)}</div>`,
			want: want{
				code: "${$$maybeRenderHead($$result)}<div>${x.map((x) => x ? $$render`<div>${true ? $$render`<span>${x}</span>` : null}</div>` : $$render`<div>${false ? null : $$render`<span>${x}</span>`}</div>`)}</div>",
			},
		},
		{
			name:   "nested expressions IV",
			source: `<div>{() => { if (value > 0.25) { return <span>Default</span> } else if (value > 0.5) { return <span>Another</span> } else if (value > 0.75) { return <span>Other</span> } return <span>Yet Other</span> }}</div>`,
			want: want{
				code: "${$$maybeRenderHead($$result)}<div>${() => { if (value > 0.25) { return $$render`<span>Default</span>` } else if (value > 0.5) { return $$render`<span>Another</span>` } else if (value > 0.75) { return $$render`<span>Other</span>` } return $$render`<span>Yet Other</span>` }}</div>",
			},
		},
		{
			name:   "nested expressions V",
			source: `<div><h1>title</h1>{list.map(group => <Fragment><h2>{group.label}</h2>{group.items.map(item => <span>{item}</span>)}</Fragment>)}</div>`,
			want: want{
				code: "${$$maybeRenderHead($$result)}<div><h1>title</h1>${list.map(group => $$render`${$$renderComponent($$result,'Fragment',Fragment,{},{\"default\": () => $$render`<h2>${group.label}</h2>${group.items.map(item => $$render`<span>${item}</span>`)}`,})}`)}</div>",
			},
		},
		{
			name:   "nested expressions VI",
			source: `<div>{()=>{ if (true) { return <hr />;} if (true) { return <img />;}}}</div>`,
			want: want{
				code: "${$$maybeRenderHead($$result)}<div>${()=>{ if (true) { return $$render`<hr>`;} if (true) { return $$render`<img>`;}}}</div>",
			},
		},
		{
			name:   "nested expressions VII",
			source: `<div>{() => { if (value > 0.25) { return <br />;} else if (value > 0.5) { return <hr />;} else if (value > 0.75) { return <div />;} return <div>Yaaay</div>;}</div>`,
			want: want{
				code: "${$$maybeRenderHead($$result)}<div>${() => { if (value > 0.25) { return $$render`<br>`;} else if (value > 0.5) { return $$render`<hr>`;} else if (value > 0.75) { return $$render`<div></div>`;} return $$render`<div>Yaaay</div>`;}}</div>",
			},
		},
		{
			name:   "nested expressions VIII",
			source: `<div>{ items.map(({ type, ...data }) => { switch (type) { case 'card': { return ( <Card {...data} /> ); } case 'paragraph': { return ( <p>{data.body}</p>);}}})}</div>`,
			want: want{
				code: "${$$maybeRenderHead($$result)}<div>${ items.map(({ type, ...data }) => { switch (type) { case 'card': { return ( $$render`${$$renderComponent($$result,'Card',Card,{...(data)})}` ); } case 'paragraph': { return ( $$render`<p>${data.body}</p>`);}}})}</div>",
			},
		},
		{
			name: "expressions with JS comments",
			source: `---
const items = ['red', 'yellow', 'blue'];
---
<div>
  {items.map((item) => (
    // foo < > < }
    <div id={color}>color</div>
  ))}
  {items.map((item) => (
    /* foo < > < } */ <div id={color}>color</div>
  ))}
</div>`,
			want: want{
				frontmatter: []string{"", "const items = ['red', 'yellow', 'blue'];"},
				code: `${$$maybeRenderHead($$result)}<div>
  ${items.map((item) => (
    // foo < > < }
    $$render` + "`" + `<div${$$addAttribute(color, "id")}>color</div>` + "`" + `
  ))}
  ${items.map((item) => (
    /* foo < > < } */ $$render` + "`" + `<div${$$addAttribute(color, "id")}>color</div>` + "`" + `
  ))}
</div>`,
			},
		},
		{
			name: "expressions with multiple curly braces",
			source: `
<div>
{
	() => {
		let generate = (input) => {
			let a = () => { return; };
			let b = () => { return; };
			let c = () => { return; };
		};
	}
}
</div>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div>
${
	() => {
		let generate = (input) => {
			let a = () => { return; };
			let b = () => { return; };
			let c = () => { return; };
		};
	}
}
</div>`,
			},
		},
		{
			name: "slots (basic)",
			source: `---
import Component from "test";
---
<Component>
	<div>Default</div>
	<div slot="named">Named</div>
</Component>`,
			want: want{
				frontmatter: []string{`import Component from "test";`},
				metadata:    metadata{modules: []string{`{ module: $$module1, specifier: 'test', assert: {} }`}},
				code:        `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + "`" + `	${$$maybeRenderHead($$result)}<div>Default</div>` + "`" + `,"named": () => $$render` + "`" + `<div>Named</div>` + "`" + `,})}`,
			},
		},
		{
			name: "slots (no comments)",
			source: `---
import Component from 'test';
---
<Component>
	<div>Default</div>
	<!-- A comment! -->
	<div slot="named">Named</div>
</Component>`,
			want: want{
				frontmatter: []string{`import Component from 'test';`},
				metadata:    metadata{modules: []string{`{ module: $$module1, specifier: 'test', assert: {} }`}},
				code:        `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + "`" + `	${$$maybeRenderHead($$result)}<div>Default</div>` + "`" + `,"named": () => $$render` + "`" + `<div>Named</div>` + "`" + `,})}`,
			},
		},
		{
			name: "slots (expression)",
			source: `
<Component {data}>
	{items.map(item => <div>{item}</div>)}
</Component>`,
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{"data":(data)},{"default": () => $$render` + BACKTICK + `${items.map(item => $$render` + BACKTICK + `${$$maybeRenderHead($$result)}<div>${item}</div>` + BACKTICK + `)}` + BACKTICK + `,})}`,
			},
		},
		{
			name: "head expression",
			source: `---
const name = "world";
---
<html>
  <head>
    <title>Hello {name}</title>
  </head>
  <body>
    <div></div>
  </body>
</html>`,
			want: want{
				frontmatter: []string{``, `const name = "world";`},
				code: `<html>
  <head>
    <title>Hello ${name}</title>
  ` + RENDER_HEAD_RESULT + `</head>
  <body>
    <div></div>
  </body></html>
  `,
			},
		},
		{
			name: "head expression and conditional rendering of fragment",
			source: `---
const testBool = true;
---
<html>
	<head>
		<meta charset="UTF-8" />
		<title>{testBool ? "Hey" : "Bye"}</title>
		{testBool && (<><meta name="description" content="test" /></>)}
	</head>
	<body>
	  <div></div>
	</body>
</html>`,
			want: want{
				frontmatter: []string{``, `const testBool = true;`},
				code: `<html>
	<head>
		<meta charset="UTF-8">
		<title>${testBool ? "Hey" : "Bye"}</title>
		${testBool && ($$render` + BACKTICK + `${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `<meta name="description" content="test">` + BACKTICK + `,})}` + BACKTICK + `)}
	` + RENDER_HEAD_RESULT + `</head>
	<body>
	  <div></div>
	</body></html>`,
			},
		},
		{
			name: "conditional rendering of title containing expression",
			source: `{
  props.title && (
    <>
      <title>{props.title}</title>
      <meta property="og:title" content={props.title} />
      <meta name="twitter:title" content={props.title} />
    </>
  )
}`,
			want: want{
				code: `${
  props.title && (
    $$render` + BACKTICK + `${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `
      <title>${props.title}</title>
      <meta property="og:title"${$$addAttribute(props.title, "content")}>
      <meta name="twitter:title"${$$addAttribute(props.title, "content")}>
    ` + BACKTICK + `,})}` + BACKTICK + `
  )
}`,
			},
		},
		{
			name: "styles (no frontmatter)",
			source: `<style>
		  .title {
		    font-family: fantasy;
		    font-size: 28px;
		  }

		  .body {
		    font-size: 1em;
		  }
		</style>

		<h1 class="title">Page Title</h1>
		<p class="body">I’m a page</p>`,
			want: want{
				code: "\n\n\t\t" + `${$$maybeRenderHead($$result)}<h1 class="title astro-dpohflym">Page Title</h1>
		<p class="body astro-dpohflym">I’m a page</p>`,
			},
		},
		{
			name: "html5 boilerplate",
			source: `<!doctype html>

<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">

  <title>A Basic HTML5 Template</title>
  <meta name="description" content="A simple HTML5 Template for new projects.">
  <meta name="author" content="SitePoint">

  <meta property="og:title" content="A Basic HTML5 Template">
  <meta property="og:type" content="website">
  <meta property="og:url" content="https://www.sitepoint.com/a-basic-html5-template/">
  <meta property="og:description" content="A simple HTML5 Template for new projects.">
  <meta property="og:image" content="image.png">

  <link rel="icon" href="/favicon.ico">
  <link rel="icon" href="/favicon.svg" type="image/svg+xml">
  <link rel="apple-touch-icon" href="/apple-touch-icon.png">

  <link rel="stylesheet" href="css/styles.css?v=1.0">

</head>

<body>
  <!-- your content here... -->
  <script is:inline src="js/scripts.js"></script>
  </body>
</html>`,
			want: want{
				code: `<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">

  <title>A Basic HTML5 Template</title>
  <meta name="description" content="A simple HTML5 Template for new projects.">
  <meta name="author" content="SitePoint">

  <meta property="og:title" content="A Basic HTML5 Template">
  <meta property="og:type" content="website">
  <meta property="og:url" content="https://www.sitepoint.com/a-basic-html5-template/">
  <meta property="og:description" content="A simple HTML5 Template for new projects.">
  <meta property="og:image" content="image.png">

  <link rel="icon" href="/favicon.ico">
  <link rel="icon" href="/favicon.svg" type="image/svg+xml">
  <link rel="apple-touch-icon" href="/apple-touch-icon.png">

  <link rel="stylesheet" href="css/styles.css?v=1.0">

` + RENDER_HEAD_RESULT + `</head>

<body>
  <!-- your content here... -->
  <script src="js/scripts.js"></script>
  </body>
</html>`,
			},
		},
		{
			name: "React framework example",
			source: `---
// Component Imports
import Counter from '../components/Counter.jsx'
const someProps = {
  count: 0,
}

// Full Astro Component Syntax:
// https://docs.astro.build/core-concepts/astro-components/
---
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta
      name="viewport"
      content="width=device-width"
    />
    <link rel="icon" type="image/x-icon" href="/favicon.ico" />
    <style>
      :global(:root) {
        font-family: system-ui;
        padding: 2em 0;
      }
      :global(.counter) {
        display: grid;
        grid-template-columns: repeat(3, minmax(0, 1fr));
        place-items: center;
        font-size: 2em;
        margin-top: 2em;
      }
      :global(.children) {
        display: grid;
        place-items: center;
        margin-bottom: 2em;
      }
    </style>
  </head>
  <body>
    <main>
      <Counter {...someProps} client:visible>
        <h1>Hello React!</h1>
      </Counter>
    </main>
  </body>
</html>`,
			want: want{
				frontmatter: []string{`import Counter from '../components/Counter.jsx'`,
					`// Component Imports
const someProps = {
  count: 0,
}

// Full Astro Component Syntax:
// https://docs.astro.build/core-concepts/astro-components/`},
				metadata: metadata{
					modules:             []string{`{ module: $$module1, specifier: '../components/Counter.jsx', assert: {} }`},
					hydratedComponents:  []string{`Counter`},
					hydrationDirectives: []string{"visible"},
				},
				code: `<html lang="en" class="astro-hmnnhvcq">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width">
    <link rel="icon" type="image/x-icon" href="/favicon.ico">

  ` + RENDER_HEAD_RESULT + `</head>
  <body class="astro-hmnnhvcq">
    <main class="astro-hmnnhvcq">
      ${$$renderComponent($$result,'Counter',Counter,{...(someProps),"client:visible":true,"client:component-hydration":"visible","client:component-path":("../components/Counter.jsx"),"client:component-export":("default"),"class":"astro-hmnnhvcq"},{"default": () => $$render` + "`" + `
        <h1 class="astro-hmnnhvcq">Hello React!</h1>
      ` + "`" + `,})}
    </main>
  </body></html>
  `,
			},
		},
		{
			name: "script in <head>",
			source: `---
import Widget from '../components/Widget.astro';
import Widget2 from '../components/Widget2.astro';
---
<html lang="en">
  <head>
    <script type="module" src="/regular_script.js"></script>
  </head>`,
			want: want{
				frontmatter: []string{`import Widget from '../components/Widget.astro';
import Widget2 from '../components/Widget2.astro';`},
				metadata: metadata{
					modules: []string{
						`{ module: $$module1, specifier: '../components/Widget.astro', assert: {} }`,
						`{ module: $$module2, specifier: '../components/Widget2.astro', assert: {} }`},
				},
				code: `<html lang="en">
  <head>
    <script type="module" src="/regular_script.js"></script>
  ` + RENDER_HEAD_RESULT + `</head></html>`,
			},
		},
		{
			name: "script hoist with frontmatter",
			source: `---
---
<script type="module" hoist>console.log("Hello");</script>`,
			want: want{
				frontmatter: []string{""},
				metadata:    metadata{hoisted: []string{fmt.Sprintf(`{ type: 'inline', value: %sconsole.log("Hello");%s }`, BACKTICK, BACKTICK)}},
				code:        ``,
			},
		},
		{
			name: "script hoist without frontmatter",
			source: `
							<main>
								<script type="module" hoist>console.log("Hello");</script>
							`,
			want: want{
				metadata: metadata{hoisted: []string{fmt.Sprintf(`{ type: 'inline', value: %sconsole.log("Hello");%s }`, BACKTICK, BACKTICK)}},
				code: "${$$maybeRenderHead($$result)}<main>\n" +
					"</main>",
			},
		},
		{
			name:   "script inline",
			source: `<main><script is:inline type="module">console.log("Hello");</script>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<main><script type="module">console.log("Hello");</script></main>`,
			},
		},
		{
			name:   "script define:vars I",
			source: `<script define:vars={{ value: 0 }}>console.log(value);</script>`,
			want: want{
				code: `<script>(function(){${$$defineScriptVars({ value: 0 })}console.log(value);})();</script>`,
			},
		},
		{
			name:   "script define:vars II",
			source: `<script define:vars={{ "dash-case": true }}>console.log(dashCase);</script>`,
			want: want{
				code: `<script>(function(){${$$defineScriptVars({ "dash-case": true })}console.log(dashCase);})();</script>`,
			},
		},
		{
			name:   "script before elements",
			source: `<script>Here</script><div></div>`,
			want: want{
				metadata: metadata{hoisted: []string{fmt.Sprintf(`{ type: 'inline', value: %sHere%s }`, BACKTICK, BACKTICK)}},
				code:     `${$$maybeRenderHead($$result)}<div></div>`,
			},
		},
		{
			name:   "script (renderScript: true)",
			source: `<main><script>console.log("Hello");</script>`,
			transformOptions: transform.TransformOptions{
				RenderScript: true,
			},
			filename: "/src/pages/index.astro",
			want: want{
				metadata: metadata{hoisted: []string{fmt.Sprintf(`{ type: 'inline', value: %sconsole.log("Hello");%s }`, BACKTICK, BACKTICK)}},
				code:     `${$$maybeRenderHead($$result)}<main>${$$renderScript($$result,"/src/pages/index.astro?astro&type=script&index=0&lang.ts")}</main>`,
			},
		},
		{
			name:   "script multiple (renderScript: true)",
			source: `<main><script>console.log("Hello");</script><script>console.log("World");</script>`,
			transformOptions: transform.TransformOptions{
				RenderScript: true,
			},
			filename: "/src/pages/index.astro",
			want: want{
				metadata: metadata{
					hoisted: []string{
						fmt.Sprintf(`{ type: 'inline', value: %sconsole.log("World");%s }`, BACKTICK, BACKTICK),
						fmt.Sprintf(`{ type: 'inline', value: %sconsole.log("Hello");%s }`, BACKTICK, BACKTICK),
					},
				},
				code: `${$$maybeRenderHead($$result)}<main>${$$renderScript($$result,"/src/pages/index.astro?astro&type=script&index=0&lang.ts")}${$$renderScript($$result,"/src/pages/index.astro?astro&type=script&index=1&lang.ts")}</main>`,
			},
		},
		{
			name:   "script external (renderScript: true)",
			source: `<main><script src="./hello.js"></script>`,
			transformOptions: transform.TransformOptions{
				RenderScript: true,
			},
			filename: "/src/pages/index.astro",
			want: want{
				metadata: metadata{hoisted: []string{`{ type: 'external', src: './hello.js' }`}},
				code:     `${$$maybeRenderHead($$result)}<main>${$$renderScript($$result,"/src/pages/index.astro?astro&type=script&index=0&lang.ts")}</main>`,
			},
		},
		{
			name:   "script inline (renderScript: true)",
			source: `<main><script is:inline type="module">console.log("Hello");</script>`,
			transformOptions: transform.TransformOptions{
				RenderScript: true,
			},
			want: want{
				code: `${$$maybeRenderHead($$result)}<main><script type="module">console.log("Hello");</script></main>`,
			},
		},
		{
			name:   "script mixed handled and inline (renderScript: true)",
			source: `<main><script>console.log("Hello");</script><script is:inline>console.log("World");</script>`,
			transformOptions: transform.TransformOptions{
				RenderScript: true,
			},
			filename: "/src/pages/index.astro",
			want: want{
				metadata: metadata{hoisted: []string{fmt.Sprintf(`{ type: 'inline', value: %sconsole.log("Hello");%s }`, BACKTICK, BACKTICK)}},
				code:     `${$$maybeRenderHead($$result)}<main>${$$renderScript($$result,"/src/pages/index.astro?astro&type=script&index=0&lang.ts")}<script>console.log("World");</script></main>`,
			},
		},
		{
			name:   "text after title expression",
			source: `<title>a {expr} b</title>`,
			want: want{
				code: `<title>a ${expr} b</title>`,
			},
		},
		{
			name:   "text after title expressions",
			source: `<title>a {expr} b {expr} c</title>`,
			want: want{
				code: `<title>a ${expr} b ${expr} c</title>`,
			},
		},
		{
			name: "slots (dynamic name)",
			source: `---
import Component from 'test';
const name = 'named';
		---
		<Component>
			<div slot={name}>Named</div>
		</Component>`,
			want: want{
				frontmatter: []string{`import Component from 'test';`, `const name = 'named';`},
				metadata:    metadata{modules: []string{`{ module: $$module1, specifier: 'test', assert: {} }`}},
				code:        `${$$renderComponent($$result,'Component',Component,{},{[name]: () => $$render` + "`" + `${$$maybeRenderHead($$result)}<div>Named</div>` + "`" + `,})}`,
			},
		},
		{
			name: "slots (named only)",
			source: `<Slotted>
      <span slot="a">A</span>
      <span slot="b">B</span>
      <span slot="c">C</span>
    </Slotted>`,
			want: want{
				code: `${$$renderComponent($$result,'Slotted',Slotted,{},{"a": () => $$render` + BACKTICK + `${$$maybeRenderHead($$result)}<span>A</span>` + BACKTICK + `,"b": () => $$render` + BACKTICK + `<span>B</span>` + BACKTICK + `,"c": () => $$render` + BACKTICK + `<span>C</span>` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "condition expressions at the top-level",
			source: `{cond && <span></span>}{cond && <strong></strong>}`,
			want: want{
				code: "${cond && $$render`${$$maybeRenderHead($$result)}<span></span>`}${cond && $$render`<strong></strong>`}",
			},
		},
		{
			name:   "condition expressions at the top-level with head content",
			source: `{cond && <meta charset=utf8>}{cond && <title>My title</title>}`,
			want: want{
				code: "${cond && $$render`<meta charset=\"utf8\">`}${cond && $$render`<title>My title</title>`}",
			},
		},
		{
			name: "custom elements",
			source: `---
import 'test';
---
<my-element></my-element>`,
			want: want{
				frontmatter: []string{`import 'test';`},
				metadata:    metadata{modules: []string{`{ module: $$module1, specifier: 'test', assert: {} }`}},
				code:        `${$$renderComponent($$result,'my-element','my-element',{})}`,
			},
		},
		{
			name: "gets all potential hydrated components",
			source: `---
import One from 'one';
import Two from 'two';
import 'custom-element';
const name = 'world';
---
<One client:load />
<Two client:load />
<my-element client:load />
`,
			want: want{
				frontmatter: []string{`import One from 'one';
import Two from 'two';
import 'custom-element';`,
					`const name = 'world';`},
				metadata: metadata{
					modules: []string{
						`{ module: $$module1, specifier: 'one', assert: {} }`,
						`{ module: $$module2, specifier: 'two', assert: {} }`,
						`{ module: $$module3, specifier: 'custom-element', assert: {} }`,
					},
					hydratedComponents:  []string{"'my-element'", "Two", "One"},
					hydrationDirectives: []string{"load"},
				},
				code: `${$$renderComponent($$result,'One',One,{"client:load":true,"client:component-hydration":"load","client:component-path":("one"),"client:component-export":("default")})}
${$$renderComponent($$result,'Two',Two,{"client:load":true,"client:component-hydration":"load","client:component-path":("two"),"client:component-export":("default")})}
${$$renderComponent($$result,'my-element','my-element',{"client:load":true,"client:component-hydration":"load"})}`,
			},
		},
		{
			name:   "Component siblings are siblings",
			source: `<BaseHead></BaseHead><link href="test">`,
			want: want{
				code: `${$$renderComponent($$result,'BaseHead',BaseHead,{})}<link href="test">`,
			},
		},
		{
			name:   "Self-closing components siblings are siblings",
			source: `<BaseHead /><link href="test">`,
			want: want{
				code: `${$$renderComponent($$result,'BaseHead',BaseHead,{})}<link href="test">`,
			},
		},
		{
			name:   "Self-closing script in head works",
			source: `<html><head><script is:inline /></head><html>`,
			want: want{
				code: `<html><head><script></script>` + RENDER_HEAD_RESULT + `</head></html>`,
			},
		},
		{
			name:   "Self-closing title",
			source: `<title />`,
			want: want{
				code: `<title></title>`,
			},
		},
		{
			name:   "Self-closing title II",
			source: `<html><head><title /></head><body></body></html>`,
			want: want{
				code: `<html><head><title></title>` + RENDER_HEAD_RESULT + `</head><body></body></html>`,
			},
		},
		{
			name:   "Self-closing components in head can have siblings",
			source: `<html><head><BaseHead /><link href="test"></head><html>`,
			want: want{
				code: `<html><head>${$$renderComponent($$result,'BaseHead',BaseHead,{})}<link href="test">` + RENDER_HEAD_RESULT + `</head></html>`,
			},
		},
		{
			name:   "Self-closing formatting elements",
			source: `<div id="1"><div id="2"><div id="3"><i/><i/><i/></div></div></div>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div id="1"><div id="2"><div id="3"><i></i><i></i><i></i></div></div></div>`,
			},
		},
		{
			name: "Self-closing formatting elements 2",
			source: `<body>
  <div id="1"><div id="2"><div id="3"><i id="a" /></div></div></div>
  <div id="4"><div id="5"><div id="6"><i id="b" /></div></div></div>
  <div id="7"><div id="8"><div id="9"><i id="c" /></div></div></div>
</body>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<body>
  <div id="1"><div id="2"><div id="3"><i id="a"></i></div></div></div>
  <div id="4"><div id="5"><div id="6"><i id="b"></i></div></div></div>
  <div id="7"><div id="8"><div id="9"><i id="c"></i></div></div></div>
</body>`,
			},
		},
		{
			name: "Nested HTML in expressions, wrapped in parens",
			source: `---
const image = './penguin.png';
const canonicalURL = new URL('http://example.com');
---
{image && (<meta property="og:image" content={new URL(image, canonicalURL)}>)}`,
			want: want{
				frontmatter: []string{"", `const image = './penguin.png';
const canonicalURL = new URL('http://example.com');`},
				code: "${image && ($$render`<meta property=\"og:image\"${$$addAttribute(new URL(image, canonicalURL), \"content\")}>`)}",
			},
		},
		{
			name: "Use of interfaces within frontmatter",
			source: `---
interface MarkdownFrontmatter {
	date: number;
	image: string;
	author: string;
}
let allPosts = Astro.fetchContent<MarkdownFrontmatter>('./post/*.md');
---
<div>testing</div>`,
			want: want{
				frontmatter: []string{"", `interface MarkdownFrontmatter {
	date: number;
	image: string;
	author: string;
}
let allPosts = Astro.fetchContent<MarkdownFrontmatter>('./post/*.md');`},
				code: "${$$maybeRenderHead($$result)}<div>testing</div>",
			},
		},
		{
			name: "dynamic import",
			source: `---
const markdownDocs = await Astro.glob('../markdown/*.md')
const article2 = await import('../markdown/article2.md')
---
<div />
`, want: want{
				frontmatter: []string{"", `const markdownDocs = await Astro.glob('../markdown/*.md')
const article2 = await import('../markdown/article2.md')`},
				code: "${$$maybeRenderHead($$result)}<div></div>",
			},
		},
		{
			name: "Component names A-Z",
			source: `---
import AComponent from '../components/AComponent.jsx';
import ZComponent from '../components/ZComponent.jsx';
---

<body>
  <AComponent />
  <ZComponent />
</body>`,
			want: want{
				frontmatter: []string{
					`import AComponent from '../components/AComponent.jsx';
import ZComponent from '../components/ZComponent.jsx';`},
				metadata: metadata{
					modules: []string{
						`{ module: $$module1, specifier: '../components/AComponent.jsx', assert: {} }`,
						`{ module: $$module2, specifier: '../components/ZComponent.jsx', assert: {} }`,
					},
				},
				code: `${$$maybeRenderHead($$result)}<body>
  ${` + RENDER_COMPONENT + `($$result,'AComponent',AComponent,{})}
  ${` + RENDER_COMPONENT + `($$result,'ZComponent',ZComponent,{})}
</body>`,
			},
		},
		{
			name: "Parser can handle files > 4096 chars",
			source: `<html><body>` + longRandomString + `<img
  width="1600"
  height="1131"
  class="img"
  src="https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1200&q=75"
  srcSet="https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1200&q=75 800w,https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1200&q=75 1200w,https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1600&q=75 1600w,https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=2400&q=75 2400w"
  sizes="(max-width: 800px) 800px, (max-width: 1200px) 1200px, (max-width: 1600px) 1600px, (max-width: 2400px) 2400px, 1200px"
>`,
			want: want{
				code: `<html>${$$maybeRenderHead($$result)}<body>` + longRandomString + `<img width="1600" height="1131" class="img" src="https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1200&q=75" srcSet="https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1200&q=75 800w,https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1200&q=75 1200w,https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1600&q=75 1600w,https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=2400&q=75 2400w" sizes="(max-width: 800px) 800px, (max-width: 1200px) 1200px, (max-width: 1600px) 1600px, (max-width: 2400px) 2400px, 1200px"></body></html>`,
			},
		},
		{
			name:   "SVG styles",
			source: `<svg><style>path { fill: red; }</style></svg>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<svg><style>path { fill: red; }</style></svg>`,
			},
		},
		{
			name: "svg expressions",
			source: `---
const title = 'icon';
---
<svg>{title ?? null}</svg>`,
			want: want{
				frontmatter: []string{"", "const title = 'icon';"},
				code:        `${$$maybeRenderHead($$result)}<svg>${title ?? null}</svg>`,
			},
		},
		{
			name: "advanced svg expression",
			source: `---
const title = 'icon';
---
<svg>{title ? <title>{title}</title> : null}</svg>`,
			want: want{
				frontmatter: []string{"", "const title = 'icon';"},
				code:        `${$$maybeRenderHead($$result)}<svg>${title ? $$render` + BACKTICK + `<title>${title}</title>` + BACKTICK + ` : null}</svg>`,
			},
		},
		{
			name:   "Empty script",
			source: `<script hoist></script>`,
			want: want{
				code: ``,
			},
		},
		{
			name:   "Empty style",
			source: `<style define:vars={{ color: "Gainsboro" }}></style>`,
			want: want{
				definedVars: []string{`{ color: "Gainsboro" }`},
				code:        ``,
			},
		},
		{
			name: "No extra script tag",
			source: `<!-- Global Metadata -->
<meta charset="utf-8">
<meta name="viewport" content="width=device-width">

<link rel="icon" type="image/svg+xml" href="/favicon.svg" />
<link rel="alternate icon" type="image/x-icon" href="/favicon.ico" />

<link rel="sitemap" href="/sitemap.xml"/>

<!-- Global CSS -->
<link rel="stylesheet" href="/theme.css" />
<link rel="stylesheet" href="/code.css" />
<link rel="stylesheet" href="/index.css" />

<!-- Preload Fonts -->
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:ital@0;1&display=swap" rel="stylesheet">

<!-- Scrollable a11y code helper -->
<script type="module" src="/make-scrollable-code-focusable.js" />

<!-- This is intentionally inlined to avoid FOUC -->
<script is:inline>
  const root = document.documentElement;
  const theme = localStorage.getItem('theme');
  if (theme === 'dark' || (!theme) && window.matchMedia('(prefers-color-scheme: dark)').matches) {
    root.classList.add('theme-dark');
  } else {
    root.classList.remove('theme-dark');
  }
</script>

<!-- Global site tag (gtag.js) - Google Analytics -->
<!-- <script async src="https://www.googletagmanager.com/gtag/js?id=G-TEL60V1WM9"></script>
<script>
  window.dataLayer = window.dataLayer || [];
  function gtag(){dataLayer.push(arguments);}
  gtag('js', new Date());
  gtag('config', 'G-TEL60V1WM9');
</script> -->`,
			want: want{
				code: `<!-- Global Metadata --><meta charset="utf-8">
<meta name="viewport" content="width=device-width">

<link rel="icon" type="image/svg+xml" href="/favicon.svg">
<link rel="alternate icon" type="image/x-icon" href="/favicon.ico">

<link rel="sitemap" href="/sitemap.xml">

<!-- Global CSS -->
<link rel="stylesheet" href="/theme.css">
<link rel="stylesheet" href="/code.css">
<link rel="stylesheet" href="/index.css">

<!-- Preload Fonts -->
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:ital@0;1&display=swap" rel="stylesheet">

<!-- Scrollable a11y code helper -->
<script type="module" src="/make-scrollable-code-focusable.js"></script>

<!-- This is intentionally inlined to avoid FOUC -->
<script>
  const root = document.documentElement;
  const theme = localStorage.getItem('theme');
  if (theme === 'dark' || (!theme) && window.matchMedia('(prefers-color-scheme: dark)').matches) {
    root.classList.add('theme-dark');
  } else {
    root.classList.remove('theme-dark');
  }
</script>

<!-- Global site tag (gtag.js) - Google Analytics -->
<!-- <script async src="https://www.googletagmanager.com/gtag/js?id=G-TEL60V1WM9"></script>
<script>
  window.dataLayer = window.dataLayer || [];
  function gtag(){dataLayer.push(arguments);}
  gtag('js', new Date());
  gtag('config', 'G-TEL60V1WM9');
</script> -->`,
			},
		},
		{
			name: "All components",
			source: `
---
import { Container, Col, Row } from 'react-bootstrap';
---
<Container>
    <Row>
        <Col>
            <h1>Hi!</h1>
        </Col>
    </Row>
</Container>
`,
			want: want{
				frontmatter: []string{`import { Container, Col, Row } from 'react-bootstrap';`},
				metadata:    metadata{modules: []string{`{ module: $$module1, specifier: 'react-bootstrap', assert: {} }`}},
				code:        "${$$renderComponent($$result,'Container',Container,{},{\"default\": () => $$render`\n    ${$$renderComponent($$result,'Row',Row,{},{\"default\": () => $$render`\n        ${$$renderComponent($$result,'Col',Col,{},{\"default\": () => $$render`\n            ${$$maybeRenderHead($$result)}<h1>Hi!</h1>\n        `,})}\n    `,})}`,})}",
			},
		},
		{
			name: "Mixed style siblings",
			source: `<head>
	<style is:global>div { color: red }</style>
	<style is:scoped>div { color: green }</style>
	<style>div { color: blue }</style>
</head>
<div />`,
			want: want{
				code: "<head>\n\n\n\n\n\n\n" + RENDER_HEAD_RESULT + "</head>\n<div class=\"astro-lasntlja\"></div>",
			},
		},
		{
			name:   "spread with double quotation marks",
			source: `<div {...propsFn("string")}/>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div${$$spreadAttributes(propsFn("string"))}></div>`,
			},
		},
		{
			name:   "class with spread",
			source: `<div class="something" {...Astro.props} />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div class="something"${$$spreadAttributes(Astro.props)}></div>`,
			},
		},
		{
			name:   "class:list with spread",
			source: `<div class:list="something" {...Astro.props} />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div class:list="something"${$$spreadAttributes(Astro.props)}></div>`,
			},
		},
		{
			name:   "class list",
			source: `<div class:list={['one', 'variable']} />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div${$$addAttribute(['one', 'variable'], "class:list")}></div>`,
			},
		},
		{
			name:   "class and class list simple array",
			source: `<div class="two" class:list={['one', 'variable']} />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div${$$addAttribute(['two', ['one', 'variable']], "class:list")}></div>`,
			},
		},
		{
			name:   "class and class list object",
			source: `<div class="two three" class:list={['hello goodbye', { hello: true, world: true }]} />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div${$$addAttribute(['two three', ['hello goodbye', { hello: true, world: true }]], "class:list")}></div>`,
			},
		},
		{
			name:   "class and class list set",
			source: `<div class="two three" class:list={[ new Set([{hello: true, world: true}]) ]} />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div${$$addAttribute(['two three', [ new Set([{hello: true, world: true}]) ]], "class:list")}></div>`,
			},
		},
		{
			name:   "spread without style or class",
			source: `<div {...Astro.props} />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div${$$spreadAttributes(Astro.props)}></div>`,
			},
		},
		{
			name:   "spread with style but no explicit class",
			source: `<style>div { color: red; }</style><div {...Astro.props} />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div${$$spreadAttributes(Astro.props,undefined,{"class":"astro-XXXX"})}></div>`,
			},
		},
		{
			name:   "Fragment",
			source: `<body><Fragment><div>Default</div><div>Named</div></Fragment></body>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<body>${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `<div>Default</div><div>Named</div>` + BACKTICK + `,})}</body>`,
			},
		},
		{
			name:   "Fragment shorthand",
			source: `<body><><div>Default</div><div>Named</div></></body>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<body>${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `<div>Default</div><div>Named</div>` + BACKTICK + `,})}</body>`,
			},
		},
		{
			name:   "Fragment shorthand only",
			source: `<>Hello</>`,
			want: want{
				code: `${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `Hello` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "Fragment literal only",
			source: `<Fragment>world</Fragment>`,
			want: want{
				code: `${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `world` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "Fragment slotted",
			source: `<body><Component><><div>Default</div><div>Named</div></></Component></body>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<body>${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + BACKTICK + `${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `<div>Default</div><div>Named</div>` + BACKTICK + `,})}` + BACKTICK + `,})}</body>`,
			},
		},
		{
			name:   "Fragment slotted with name",
			source: `<body><Component><Fragment slot=named><div>Default</div><div>Named</div></Fragment></Component></body>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<body>${$$renderComponent($$result,'Component',Component,{},{"named": () => $$render` + BACKTICK + `${$$renderComponent($$result,'Fragment',Fragment,{"slot":"named"},{"default": () => $$render` + BACKTICK + `<div>Default</div><div>Named</div>` + BACKTICK + `,})}` + BACKTICK + `,})}</body>`,
			},
		},
		{
			name:   "Preserve slots inside custom-element",
			source: `<body><my-element><div slot=name>Name</div><div>Default</div></my-element></body>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<body>${$$renderComponent($$result,'my-element','my-element',{},{"default": () => $$render` + BACKTICK + `<div slot="name">Name</div><div>Default</div>` + BACKTICK + `,})}</body>`,
			},
		},
		{
			name:   "Preserve namespaces",
			source: `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><rect xlink:href="#id"></svg>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><rect xlink:href="#id"></rect></svg>`,
			},
		},
		{
			name:   "Preserve namespaces in expressions",
			source: `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><rect xlink:href={` + BACKTICK + `#${iconId}` + BACKTICK + `}></svg>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><rect ${$$addAttribute(` + BACKTICK + `#${iconId}` + BACKTICK + `, "xlink:href")}></rect></svg>`,
			},
		},
		{
			name:   "Preserve namespaces for components",
			source: `<Component some:thing="foobar">`,
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{"some:thing":"foobar"})}`,
			},
		},
		{
			name: "import.meta.env",
			source: fmt.Sprintf(`---
import Header from '../../components/Header.jsx'
import Footer from '../../components/Footer.astro'
import ProductPageContent from '../../components/ProductPageContent.jsx';

export async function getStaticPaths() {
  let products = await fetch(%s${import.meta.env.PUBLIC_NETLIFY_URL}/.netlify/functions/get-product-list%s)
    .then(res => res.json()).then((response) => {
      console.log('--- built product pages ---')
      return response.products.edges
    });

  return products.map((p, i) => {
    return {
      params: {pid: p.node.handle},
      props: {product: p},
    };
  });
}

const { product } = Astro.props;
---

<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Shoperoni | Buy {product.node.title}</title>

  <link rel="icon" type="image/svg+xml" href="/favicon.svg">
  <link rel="stylesheet" href="/style/global.css">
</head>
<body>
  <Header />
  <div class="product-page">
    <article>
      <ProductPageContent client:visible product={product.node} />
    </article>
  </div>
  <Footer />
</body>
</html>`, BACKTICK, BACKTICK),
			want: want{
				code: `<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Shoperoni | Buy ${product.node.title}</title>

  <link rel="icon" type="image/svg+xml" href="/favicon.svg">
  <link rel="stylesheet" href="/style/global.css">
` + RENDER_HEAD_RESULT + `</head>
<body>
  ${$$renderComponent($$result,'Header',Header,{})}
  <div class="product-page">
    <article>
      ${$$renderComponent($$result,'ProductPageContent',ProductPageContent,{"client:visible":true,"product":(product.node),"client:component-hydration":"visible","client:component-path":("../../components/ProductPageContent.jsx"),"client:component-export":("default")})}
    </article>
  </div>
  ${$$renderComponent($$result,'Footer',Footer,{})}
</body></html>
`,
				frontmatter: []string{
					`import Header from '../../components/Header.jsx'
import Footer from '../../components/Footer.astro'
import ProductPageContent from '../../components/ProductPageContent.jsx';`,
					"const { product } = Astro.props;",
				},
				getStaticPaths: fmt.Sprintf(`export async function getStaticPaths() {
  let products = await fetch(%s${import.meta.env.PUBLIC_NETLIFY_URL}/.netlify/functions/get-product-list%s)
    .then(res => res.json()).then((response) => {
      console.log('--- built product pages ---')
      return response.products.edges
    });

  return products.map((p, i) => {
    return {
      params: {pid: p.node.handle},
      props: {product: p},
    };
  });
}`, BACKTICK, BACKTICK),
				metadata: metadata{
					modules: []string{`{ module: $$module1, specifier: '../../components/Header.jsx', assert: {} }`,
						`{ module: $$module2, specifier: '../../components/Footer.astro', assert: {} }`,
						`{ module: $$module3, specifier: '../../components/ProductPageContent.jsx', assert: {} }`,
					},
					hydratedComponents:  []string{`ProductPageContent`},
					hydrationDirectives: []string{"visible"},
				},
			},
		},
		{
			name: "import.meta",
			source: `---
const components = import.meta.glob("../components/*.astro", {
  import: 'default'
});
---`,
			want: want{
				frontmatter: []string{"", `const components = import.meta.glob("../components/*.astro", {
  import: 'default'
});`},
			},
		},
		{
			name:   "doctype",
			source: `<!DOCTYPE html><div/>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div></div>`,
			},
		},
		{
			name: "select option expression",
			source: `---
const value = 'test';
---
<select><option>{value}</option></select>`,
			want: want{
				frontmatter: []string{"", "const value = 'test';"},
				code:        `${$$maybeRenderHead($$result)}<select><option>${value}</option></select>`,
			},
		},
		{
			name: "select nested option",
			source: `---
const value = 'test';
---
<select>{value && <option>{value}</option>}</select>`,
			want: want{
				frontmatter: []string{"", "const value = 'test';"},
				code:        `${$$maybeRenderHead($$result)}<select>${value && $$render` + BACKTICK + `<option>${value}</option>` + BACKTICK + `}</select>`,
			},
		},
		{
			name:   "select map expression",
			source: `<select>{[1, 2, 3].map(num => <option>{num}</option>)}</select><div>Hello world!</div>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<select>${[1, 2, 3].map(num => $$render` + BACKTICK + `<option>${num}</option>` + BACKTICK + `)}</select><div>Hello world!</div>`,
			},
		},
		{
			name: "textarea",
			source: `---
const value = 'test';
---
<textarea>{value}</textarea>`,
			want: want{
				frontmatter: []string{"", "const value = 'test';"},
				code:        `${$$maybeRenderHead($$result)}<textarea>${value}</textarea>`,
			},
		},
		{
			name:   "textarea inside expression",
			source: `{bool && <textarea>{value}</textarea>} {!bool && <input>}`,
			want: want{
				code: `${bool && $$render` + BACKTICK + `${$$maybeRenderHead($$result)}<textarea>${value}</textarea>` + BACKTICK + `} ${!bool && $$render` + BACKTICK + `<input>` + BACKTICK + `}`,
			},
		},
		{
			name: "table simple case",
			source: `---
const content = "lol";
---

<html>
  <body>
    <table>
      <tr>
        <td>{content}</td>
      </tr>
      {
        (
          <tr>
            <td>1</td>
          </tr>
        )
      }
    </table>Hello
  </body>
</html>
`,
			want: want{
				frontmatter: []string{"", `const content = "lol";`},
				code: `<html>
  ${$$maybeRenderHead($$result)}<body>
    <table>
      <tr>
        <td>${content}</td>
      </tr>
      ${
        (
          $$render` + BACKTICK + `<tr>
            <td>1</td>
          </tr>` + BACKTICK + `
        )
      }
    </table>Hello
  </body>
</html>`,
			},
		},
		{
			name: "complex table",
			source: `<html lang="en">
    <head>
        <meta charset="UTF-8" />
        <meta name="viewport" content="width=device-width" />
        <title>Astro Multi Table</title>
    </head>
    <body>
        <main>
            <section>
                {Array(3).fill(false).map((item, idx) => <div>
                    <div class="row">
                        {'a'}
                        <table>
                            <thead>
                                <tr>
                                    <>{Array(7).fill(false).map((entry, index) => <th>A</th>)}</>
                                </tr>
                            </thead>
                            <tbody>
                                <tr><td></td></tr>
                            </tbody>
                        </table>
                    </div>
                </div>)}
            </section>
            <section>
                <div class="row">
                    <table>
                        <thead>
                            <tr>
                                <th>B</th>
                                <th>B</th>
                                <th>B</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr><td></td></tr>
                        </tbody>
                    </table>
                </div>
            </section>
        </main>
    </body>
</html>`,
			want: want{
				code: `<html lang="en">
    <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width">
        <title>Astro Multi Table</title>
    ${$$renderHead($$result)}</head>
    <body>
        <main>
            <section>
                ${Array(3).fill(false).map((item, idx) => $$render` + BACKTICK + `<div>
                    <div class="row">
                        ${'a'}
                        <table>
                            <thead>
                                <tr>
                                    ${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `${Array(7).fill(false).map((entry, index) => $$render` + BACKTICK + `<th>A</th>` + BACKTICK + `)}` + BACKTICK + `,})}
                                </tr>
                            </thead>
                            <tbody>
                                <tr><td></td></tr>
                            </tbody>
                        </table>
                    </div>
                </div>` + BACKTICK + `)}
            </section>
            <section>
                <div class="row">
                    <table>
                        <thead>
                            <tr>
                                <th>B</th>
                                <th>B</th>
                                <th>B</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr><td></td></tr>
                        </tbody>
                    </table>
                </div>
            </section>
        </main>
    </body>
</html>`,
			},
		},
		{
			name: "table with expression in 'th'",
			source: `---
const { title, footnotes, tables } = Astro.props;

interface Table {
	title: string;
	data: any[];
	showTitle: boolean;
	footnotes: string;
}
console.log(tables);
---

<div>
	<div>
	<h2>
		{title}
	</h2>
	{
		tables.map((table: Table) => (
		<>
			<div>
			<h3 class="text-3xl sm:text-5xl font-bold">{table.title}</h3>
			<table>
				<thead>
				{Object.keys(table.data[0]).map((thead) => (
					<th>{thead}</th>
				))}
				</thead>
				<tbody>
				{table.data.map((trow) => (
					<tr>
					{Object.values(trow).map((cell, index) => (
						<td>
						{cell}
						</td>
					))}
					</tr>
				))}
				</tbody>
			</table>
			</div>
		</>
		))
	}
	</div>
</div>`,
			want: want{
				frontmatter: []string{``, `const { title, footnotes, tables } = Astro.props;

interface Table {
	title: string;
	data: any[];
	showTitle: boolean;
	footnotes: string;
}
console.log(tables);`},
				code: `${$$maybeRenderHead($$result)}<div>
	<div>
	<h2>
		${title}
	</h2>
	${
		tables.map((table: Table) => (
		$$render` + BACKTICK + `${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `
			<div>
			<h3 class="text-3xl sm:text-5xl font-bold">${table.title}</h3>
			<table>
				<thead>
				${Object.keys(table.data[0]).map((thead) => (
					$$render` + BACKTICK + `<th>${thead}</th>` + BACKTICK + `
				))}
				</thead>
				<tbody>
				${table.data.map((trow) => (
					$$render` + BACKTICK + `<tr>
					${Object.values(trow).map((cell, index) => (
						$$render` + BACKTICK + `<td>
						${cell}
						</td>` + BACKTICK + `
					))}
					</tr>` + BACKTICK + `
				))}
				</tbody>
			</table>
			</div>
		` + BACKTICK + `,})}` + BACKTICK + `
		))
	}
	</div>
</div>`,
			},
		},
		{
			name: "table expressions (no implicit tbody)",
			source: `---
const items = ["Dog", "Cat", "Platipus"];
---
<table>{items.map(item => (<tr><td>{item}</td></tr>))}</table>`,
			want: want{
				frontmatter: []string{"", `const items = ["Dog", "Cat", "Platipus"];`},
				code:        `${$$maybeRenderHead($$result)}<table>${items.map(item => ($$render` + BACKTICK + `<tr><td>${item}</td></tr>` + BACKTICK + `))}</table>`,
			},
		},
		{
			name:   "table caption expression",
			source: `<table><caption>{title}</caption><tr><td>Hello</td></tr></table>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<table><caption>${title}</caption><tr><td>Hello</td></tr></table>`,
			},
		},
		{
			name:   "table expression with trailing div",
			source: `<table><tr><td>{title}</td></tr></table><div>Div</div>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<table><tr><td>${title}</td></tr></table><div>Div</div>`,
			},
		},
		{
			name: "tbody expressions",
			source: `---
const items = ["Dog", "Cat", "Platipus"];
---
<table><tr><td>Name</td></tr>{items.map(item => (<tr><td>{item}</td></tr>))}</table>`,
			want: want{
				frontmatter: []string{"", `const items = ["Dog", "Cat", "Platipus"];`},
				code:        `${$$maybeRenderHead($$result)}<table><tr><td>Name</td></tr>${items.map(item => ($$render` + BACKTICK + `<tr><td>${item}</td></tr>` + BACKTICK + `))}</table>`,
			},
		},
		{
			name: "tbody expressions 2",
			source: `---
const items = ["Dog", "Cat", "Platipus"];
---
<table><tr><td>Name</td></tr>{items.map(item => (<tr><td>{item}</td><td>{item + 's'}</td></tr>))}</table>`,
			want: want{
				frontmatter: []string{"", `const items = ["Dog", "Cat", "Platipus"];`},
				code:        `${$$maybeRenderHead($$result)}<table><tr><td>Name</td></tr>${items.map(item => ($$render` + BACKTICK + `<tr><td>${item}</td><td>${item + 's'}</td></tr>` + BACKTICK + `))}</table>`,
			},
		},
		{
			name:   "tbody expressions 3",
			source: `<table><tbody>{rows.map(row => (<tr><td><strong>{row}</strong></td></tr>))}</tbody></table>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<table><tbody>${rows.map(row => ($$render` + BACKTICK + `<tr><td><strong>${row}</strong></td></tr>` + BACKTICK + `))}</tbody></table>`,
			},
		},
		{
			name:   "td expressions",
			source: `<table><tr><td><h2>Row 1</h2></td><td>{title}</td></tr></table>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<table><tr><td><h2>Row 1</h2></td><td>${title}</td></tr></table>`,
			},
		},
		{
			name:   "td expressions II",
			source: `<table>{data.map(row => <tr>{row.map(cell => <td>{cell}</td>)}</tr>)}</table>`,
			want: want{
				code: "${$$maybeRenderHead($$result)}<table>${data.map(row => $$render`<tr>${row.map(cell => $$render`<td>${cell}</td>`)}</tr>`)}</table>",
			},
		},
		{
			name:   "self-closing td",
			source: `<table>{data.map(row => <tr>{row.map(cell => <td set:html={cell} />)}</tr>)}</table>`,
			want: want{
				code: "${$$maybeRenderHead($$result)}<table>${data.map(row => $$render`<tr>${row.map(cell => $$render`<td>${$$unescapeHTML(cell)}</td>`)}</tr>`)}</table>",
			},
		},
		{
			name:   "th expressions",
			source: `<table><thead><tr><th>{title}</th></tr></thead></table>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<table><thead><tr><th>${title}</th></tr></thead></table>`,
			},
		},
		{
			name:   "tr only",
			source: `<tr><td>col 1</td><td>col 2</td><td>{foo}</td></tr>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<tr><td>col 1</td><td>col 2</td><td>${foo}</td></tr>`,
			},
		},
		{
			name:   "caption only",
			source: `<caption>Hello world!</caption>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<caption>Hello world!</caption>`,
			},
		},
		{
			name:   "anchor expressions",
			source: `<a>{expr}</a>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<a>${expr}</a>`,
			},
		},
		{
			name:   "anchor inside expression",
			source: `{true && <a>expr</a>}`,
			want: want{
				code: `${true && $$render` + BACKTICK + `${$$maybeRenderHead($$result)}<a>expr</a>` + BACKTICK + `}`,
			},
		},
		{
			name:   "anchor content",
			source: `<a><div><h3></h3><ul><li>{expr}</li></ul></div></a>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<a><div><h3></h3><ul><li>${expr}</li></ul></div></a>`,
			},
		},
		{
			name:   "small expression",
			source: `<div><small>{a}</small>{data.map(a => <Component value={a} />)}</div>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div><small>${a}</small>${data.map(a => $$render` + BACKTICK + `${$$renderComponent($$result,'Component',Component,{"value":(a)})}` + BACKTICK + `)}</div>`,
			},
		},
		{
			name:   "division inside expression",
			source: `<div>{16 / 4}</div>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div>${16 / 4}</div>`,
			},
		},
		{
			name:   "escaped entity",
			source: `<img alt="A person saying &#x22;hello&#x22;">`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<img alt="A person saying &quot;hello&quot;">`,
			},
		},
		{
			name:   "textarea in form",
			source: `<html><Component><form><textarea></textarea></form></Component></html>`,
			want: want{
				code: `<html>${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + BACKTICK + `${$$maybeRenderHead($$result)}<form><textarea></textarea></form>` + BACKTICK + `,})}</html>`,
			},
		},
		{
			name:   "select in form",
			source: `<form><select>{options.map((option) => (<option value={option.id}>{option.title}</option>))}</select><div><label>Title 3</label><input type="text" /></div><button type="submit">Submit</button></form>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<form><select>${options.map((option) => ($$render` + BACKTICK + `<option${$$addAttribute(option.id, "value")}>${option.title}</option>` + BACKTICK + `))}</select><div><label>Title 3</label><input type="text"></div><button type="submit">Submit</button></form>`,
			},
		},
		{
			name:   "Expression in form followed by other sibling forms",
			source: "<form><p>No expression here. So the next form will render.</p></form><form><h3>{data.formLabelA}</h3></form><form><h3>{data.formLabelB}</h3></form><form><p>No expression here, but the last form before me had an expression, so my form didn't render.</p></form><form><h3>{data.formLabelC}</h3></form><div><p>Here is some in-between content</p></div><form><h3>{data.formLabelD}</h3></form>",
			want: want{
				code: "${$$maybeRenderHead($$result)}<form><p>No expression here. So the next form will render.</p></form><form><h3>${data.formLabelA}</h3></form><form><h3>${data.formLabelB}</h3></form><form><p>No expression here, but the last form before me had an expression, so my form didn't render.</p></form><form><h3>${data.formLabelC}</h3></form><div><p>Here is some in-between content</p></div><form><h3>${data.formLabelD}</h3></form>",
			},
		},
		{
			name:   "slot inside of Base",
			source: `<Base title="Home"><div>Hello</div></Base>`,
			want: want{
				code: `${$$renderComponent($$result,'Base',Base,{"title":"Home"},{"default": () => $$render` + BACKTICK + `${$$maybeRenderHead($$result)}<div>Hello</div>` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "user-defined `implicit` is printed",
			source: `<html implicit></html>`,
			want: want{
				code: `<html implicit></html>`,
			},
		},
		{
			name: "css comment doesn’t produce semicolon",
			source: `<style>/* comment */.container {
    padding: 2rem;
	}
</style><div class="container">My Text</div>`,

			want: want{
				code: `${$$maybeRenderHead($$result)}<div class="container astro-sj3wye6h">My Text</div>`,
			},
		},
		{
			name: "sibling expressions",
			source: `<html><body>
  <table>
  {true ? (<tr><td>Row 1</td></tr>) : null}
  {true ? (<tr><td>Row 2</td></tr>) : null}
  {true ? (<tr><td>Row 3</td></tr>) : null}
  </table>
</body>`,
			want: want{
				code: fmt.Sprintf(`<html>${$$maybeRenderHead($$result)}<body>
  <table>
  ${true ? ($$render%s<tr><td>Row 1</td></tr>%s) : null}
  ${true ? ($$render%s<tr><td>Row 2</td></tr>%s) : null}
  ${true ? ($$render%s<tr><td>Row 3</td></tr>%s) : null}
  </table>
</body></html>`, BACKTICK, BACKTICK, BACKTICK, BACKTICK, BACKTICK, BACKTICK),
			},
		},
		{
			name:   "table",
			source: "<table><tr>{[0,1,2].map(x => (<td>{x}</td>))}</tr></table>",
			want: want{
				code: "${$$maybeRenderHead($$result)}<table><tr>${[0,1,2].map(x => ($$render`<td>${x}</td>`))}</tr></table>",
			},
		},
		{
			name:   "table II",
			source: "<table><thead><tr>{['Hey','Ho'].map((item)=> <th scope=\"col\">{item}</th>)}</tr></thead></table>",
			want: want{
				code: "${$$maybeRenderHead($$result)}<table><thead><tr>${['Hey','Ho'].map((item)=> $$render`<th scope=\"col\">${item}</th>`)}</tr></thead></table>",
			},
		},
		{
			name:   "table III",
			source: "<table><tbody><tr><td>Cell</td><Cell /><Cell /><Cell /></tr></tbody></table>",
			want: want{
				code: "${$$maybeRenderHead($$result)}<table><tbody><tr><td>Cell</td>${$$renderComponent($$result,'Cell',Cell,{})}${$$renderComponent($$result,'Cell',Cell,{})}${$$renderComponent($$result,'Cell',Cell,{})}</tr></tbody></table>",
			},
		},
		{
			name:   "table IV",
			source: "<body><div><tr><td>hello world</td></tr></div></body>",
			want: want{
				code: "${$$maybeRenderHead($$result)}<body><div><tr><td>hello world</td></tr></div></body>",
			},
		},
		{
			name:   "table slot I",
			source: "<table><slot /></table>",
			want: want{
				code: "${$$maybeRenderHead($$result)}<table>${$$renderSlot($$result,$$slots[\"default\"])}</table>",
			},
		},
		{
			name:   "table slot II",
			source: "<table><tr><slot /></tr></table>",
			want: want{
				code: "${$$maybeRenderHead($$result)}<table><tr>${$$renderSlot($$result,$$slots[\"default\"])}</tr></table>",
			},
		},
		{
			name:   "table slot III",
			source: "<table><td><slot /></td></table>",
			want: want{
				code: "${$$maybeRenderHead($$result)}<table><td>${$$renderSlot($$result,$$slots[\"default\"])}</td></table>",
			},
		},
		{
			name:   "table slot IV",
			source: "<table><thead><slot /></thead></table>",
			want: want{
				code: "${$$maybeRenderHead($$result)}<table><thead>${$$renderSlot($$result,$$slots[\"default\"])}</thead></table>",
			},
		},
		{
			name:   "table slot V",
			source: "<table><tbody><slot /></tbody></table>",
			want: want{
				code: "${$$maybeRenderHead($$result)}<table><tbody>${$$renderSlot($$result,$$slots[\"default\"])}</tbody></table>",
			},
		},
		{
			name:   "XElement",
			source: `<XElement {...attrs}></XElement>{onLoadString ? <script data-something></script> : null }`,
			want: want{
				code: fmt.Sprintf(`${$$renderComponent($$result,'XElement',XElement,{...(attrs)})}${onLoadString ? $$render%s<script data-something></script>%s : null }`, BACKTICK, BACKTICK),
			},
		},
		{
			name:   "Empty expression",
			source: "<body>({})</body>",
			want: want{
				code: `${$$maybeRenderHead($$result)}<body>(${(void 0)})</body>`,
			},
		},
		{
			name:   "Empty expression with whitespace",
			source: "<body>({   })</body>",
			want: want{
				code: `${$$maybeRenderHead($$result)}<body>(${(void 0)   })</body>`,
			},
		},
		{
			name: "expression with leading whitespace",
			source: `<section>
<ul class="font-mono text-sm flex flex-col gap-0.5">
	{
		<li>Build: { new Date().toISOString() }</li>
		<li>NODE_VERSION: { process.env.NODE_VERSION }</li>
	}
</ul>
</section>`,
			want: want{
				code: "${$$maybeRenderHead($$result)}<section>\n<ul class=\"font-mono text-sm flex flex-col gap-0.5\">\n\t${\n\n\t\t$$render`<li>Build: ${ new Date().toISOString() }</li>\n\t\t<li>NODE_VERSION: ${ process.env.NODE_VERSION }</li>`\n\t}\n</ul>\n</section>",
			},
		},
		{
			name:   "Empty attribute expression",
			source: "<body attr={}></body>",
			want: want{
				code: `${$$maybeRenderHead($$result)}<body${$$addAttribute((void 0), "attr")}></body>`,
			},
		},
		{
			name:   "is:raw",
			source: "<article is:raw><% awesome %></article>",
			want: want{
				code: `${$$maybeRenderHead($$result)}<article><% awesome %></article>`,
			},
		},
		{
			name:   "Component is:raw",
			source: "<Component is:raw>{<% awesome %>}</Component>",
			want: want{
				code: "${$$renderComponent($$result,'Component',Component,{},{\"default\": () => $$render`{<% awesome %>}`,})}",
			},
		},
		{
			name:   "set:html",
			source: "<article set:html={content} />",
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${$$unescapeHTML(content)}</article>`,
			},
		},
		{
			name:   "set:html with quoted attribute",
			source: `<article set:html="content" />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${"content"}</article>`,
			},
		},
		{
			name:   "set:html with template literal attribute without variable",
			source: `<article set:html=` + BACKTICK + `content` + BACKTICK + ` />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${` + BACKTICK + `content` + BACKTICK + `}</article>`,
			},
		},
		{
			name:   "set:html with template literal attribute with variable",
			source: `<article set:html=` + BACKTICK + `${content}` + BACKTICK + ` />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${` + BACKTICK + `${content}` + BACKTICK + `}</article>`,
			},
		},
		{
			name:   "set:text",
			source: "<article set:text={content} />",
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${content}</article>`,
			},
		},
		{
			name:   "set:text with quoted attribute",
			source: `<article set:text="content" />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>content</article>`,
			},
		},
		{
			name:   "set:text with template literal attribute without variable",
			source: `<article set:text=` + BACKTICK + `content` + BACKTICK + ` />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${` + BACKTICK + `content` + BACKTICK + `}</article>`,
			},
		},
		{
			name:   "set:text with template literal attribute with variable",
			source: `<article set:text=` + BACKTICK + `${content}` + BACKTICK + ` />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${` + BACKTICK + `${content}` + BACKTICK + `}</article>`,
			}},
		{
			name:   "set:html on Component",
			source: `<Component set:html={content} />`,
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + "`${$$unescapeHTML(content)}`," + `})}`,
			},
		},
		{
			name:   "set:html on Component with quoted attribute",
			source: `<Component set:html="content" />`,
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + BACKTICK + `${"content"}` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "set:html on Component with template literal attribute without variable",
			source: `<Component set:html=` + BACKTICK + `content` + BACKTICK + ` />`,
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + BACKTICK + `${` + BACKTICK + `content` + BACKTICK + `}` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "set:html on Component with template literal attribute with variable",
			source: `<Component set:html=` + BACKTICK + `${content}` + BACKTICK + ` />`,
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + BACKTICK + `${` + BACKTICK + `${content}` + BACKTICK + `}` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "set:text on Component",
			source: "<Component set:text={content} />",
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + "`${content}`," + `})}`,
			},
		},
		{
			name:   "set:text on Component with quoted attribute",
			source: `<Component set:text="content" />`,
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + BACKTICK + `content` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "set:text on Component with template literal attribute without variable",
			source: `<Component set:text=` + BACKTICK + `content` + BACKTICK + ` />`,
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + BACKTICK + `${` + BACKTICK + `content` + BACKTICK + `}` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "set:text on Component with template literal attribute with variable",
			source: `<Component set:text=` + BACKTICK + `${content}` + BACKTICK + ` />`,
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + BACKTICK + `${` + BACKTICK + `${content}` + BACKTICK + `}` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "set:html on custom-element",
			source: "<custom-element set:html={content} />",
			want: want{
				code: `${$$renderComponent($$result,'custom-element','custom-element',{},{"default": () => $$render` + "`${$$unescapeHTML(content)}`," + `})}`,
			},
		},
		{
			name:   "set:html on custom-element with quoted attribute",
			source: `<custom-element set:html="content" />`,
			want: want{
				code: `${$$renderComponent($$result,'custom-element','custom-element',{},{"default": () => $$render` + BACKTICK + `${"content"}` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "set:html on custom-element with template literal attribute without variable",
			source: `<custom-element set:html=` + BACKTICK + `content` + BACKTICK + ` />`,
			want: want{
				code: `${$$renderComponent($$result,'custom-element','custom-element',{},{"default": () => $$render` + BACKTICK + `${` + BACKTICK + `content` + BACKTICK + `}` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "set:html on custom-element with template literal attribute with variable",
			source: `<custom-element set:html=` + BACKTICK + `${content}` + BACKTICK + ` />`,
			want: want{
				code: `${$$renderComponent($$result,'custom-element','custom-element',{},{"default": () => $$render` + BACKTICK + `${` + BACKTICK + `${content}` + BACKTICK + `}` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "set:text on custom-element",
			source: "<custom-element set:text={content} />",
			want: want{
				code: `${$$renderComponent($$result,'custom-element','custom-element',{},{"default": () => $$render` + "`${content}`," + `})}`,
			},
		},
		{
			name:   "set:text on custom-element with quoted attribute",
			source: `<custom-element set:text="content" />`,
			want: want{
				code: `${$$renderComponent($$result,'custom-element','custom-element',{},{"default": () => $$render` + BACKTICK + `content` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "set:text on custom-element with template literal attribute without variable",
			source: `<custom-element set:text=` + BACKTICK + `content` + BACKTICK + ` />`,
			want: want{
				code: `${$$renderComponent($$result,'custom-element','custom-element',{},{"default": () => $$render` + BACKTICK + `${` + BACKTICK + `content` + BACKTICK + `}` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "set:text on custom-element with template literal attribute with variable",
			source: `<custom-element set:text=` + BACKTICK + `${content}` + BACKTICK + ` />`,
			want: want{
				code: `${$$renderComponent($$result,'custom-element','custom-element',{},{"default": () => $$render` + BACKTICK + `${` + BACKTICK + `${content}` + BACKTICK + `}` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "set:html on self-closing tag",
			source: "<article set:html={content} />",
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${$$unescapeHTML(content)}</article>`,
			},
		},
		{
			name:   "set:html on self-closing tag with quoted attribute",
			source: `<article set:html="content" />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${"content"}</article>`,
			},
		},
		{
			name:   "set:html on self-closing tag with template literal attribute without variable",
			source: `<article set:html=` + BACKTICK + `content` + BACKTICK + ` />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${` + BACKTICK + `content` + BACKTICK + `}</article>`,
			},
		},
		{
			name:   "set:html on self-closing tag with template literal attribute with variable",
			source: `<article set:html=` + BACKTICK + `${content}` + BACKTICK + ` />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${` + BACKTICK + `${content}` + BACKTICK + `}</article>`,
			},
		},
		{
			name:   "set:html with other attributes",
			source: "<article set:html={content} cool=\"true\" />",
			want: want{
				code: `${$$maybeRenderHead($$result)}<article cool="true">${$$unescapeHTML(content)}</article>`,
			},
		},
		{
			name:   "set:html with quoted attribute and other attributes",
			source: `<article set:html="content" cool="true" />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article cool="true">${"content"}</article>`,
			},
		},
		{
			name:   "set:html with template literal attribute without variable and other attributes",
			source: `<article set:html=` + BACKTICK + `content` + BACKTICK + ` cool="true" />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article cool="true">${` + BACKTICK + `content` + BACKTICK + `}</article>`,
			},
		},
		{
			name:   "set:html with template literal attribute with variable and other attributes",
			source: `<article set:html=` + BACKTICK + `${content}` + BACKTICK + ` cool="true" />`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article cool="true">${` + BACKTICK + `${content}` + BACKTICK + `}</article>`,
			},
		},
		{
			name:   "set:html on empty tag",
			source: "<article set:html={content}></article>",
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${$$unescapeHTML(content)}</article>`,
			},
		},
		{
			name:   "set:html on empty tag with quoted attribute",
			source: `<article set:html="content"></article>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${"content"}</article>`,
			},
		},
		{
			name:   "set:html on empty tag with template literal attribute without variable",
			source: `<article set:html=` + BACKTICK + `content` + BACKTICK + `></article>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${` + BACKTICK + `content` + BACKTICK + `}</article>`,
			},
		},
		{
			name:   "set:html on empty tag with template literal attribute with variable",
			source: `<article set:html=` + BACKTICK + `${content}` + BACKTICK + `></article>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${` + BACKTICK + `${content}` + BACKTICK + `}</article>`,
			},
		},
		{
			// If both "set:*" directives are passed, we only respect the first one
			name:   "set:html and set:text",
			source: "<article set:html={content} set:text={content} />",
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${$$unescapeHTML(content)}</article>`,
			},
		},
		//
		{
			name:   "set:html on tag with children",
			source: "<article set:html={content}>!!!</article>",
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${$$unescapeHTML(content)}</article>`,
			},
		},
		{
			name:   "set:html on tag with children and quoted attribute",
			source: `<article set:html="content">!!!</article>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${"content"}</article>`,
			},
		},
		{
			name:   "set:html on tag with children and template literal attribute without variable",
			source: `<article set:html=` + BACKTICK + `content` + BACKTICK + `>!!!</article>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${` + BACKTICK + `content` + BACKTICK + `}</article>`,
			},
		},
		{
			name:   "set:html on tag with children and template literal attribute with variable",
			source: `<article set:html=` + BACKTICK + `${content}` + BACKTICK + `>!!!</article>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${` + BACKTICK + `${content}` + BACKTICK + `}</article>`,
			},
		},
		{
			name:   "set:html on tag with empty whitespace",
			source: "<article set:html={content}>   </article>",
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${$$unescapeHTML(content)}</article>`,
			},
		},
		{
			name:   "set:html on tag with empty whitespace and quoted attribute",
			source: `<article set:html="content">   </article>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${"content"}</article>`,
			},
		},
		{
			name:   "set:html on tag with empty whitespace and template literal attribute without variable",
			source: `<article set:html=` + BACKTICK + `content` + BACKTICK + `>   </article>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${` + BACKTICK + `content` + BACKTICK + `}</article>`,
			},
		},
		{
			name:   "set:html on tag with empty whitespace and template literal attribute with variable",
			source: `<article set:html=` + BACKTICK + `${content}` + BACKTICK + `>   </article>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<article>${` + BACKTICK + `${content}` + BACKTICK + `}</article>`,
			},
		},
		{
			name:   "set:html on script",
			source: "<script set:html={content} />",
			want: want{
				code: `<script>${$$unescapeHTML(content)}</script>`,
			},
		},
		{
			name:   "set:html on script with quoted attribute",
			source: `<script set:html="alert(1)" />`,
			want: want{
				code: `<script>${"alert(1)"}</script>`,
			},
		},
		{
			name:   "set:html on script with template literal attribute without variable",
			source: `<script set:html=` + BACKTICK + `alert(1)` + BACKTICK + ` />`,
			want: want{
				code: `<script>${` + BACKTICK + `alert(1)` + BACKTICK + `}</script>`,
			},
		},
		{
			name:   "set:html on script with template literal attribute with variable",
			source: `<script set:html=` + BACKTICK + `${content}` + BACKTICK + ` />`,
			want: want{
				code: `<script>${` + BACKTICK + `${content}` + BACKTICK + `}</script>`,
			},
		},
		{
			name:   "set:html on style",
			source: "<style set:html={content} />",
			want: want{
				code: `<style>${$$unescapeHTML(content)}</style>`,
			},
		},
		{
			name:   "set:html on style with quoted attribute",
			source: `<style set:html="h1{color:green;}" />`,
			want: want{
				code: `<style>${"h1{color:green;}"}</style>`,
			},
		},
		{
			name:   "set:html on style with template literal attribute without variable",
			source: `<style set:html=` + BACKTICK + `h1{color:green;}` + BACKTICK + ` />`,
			want: want{
				code: `<style>${` + BACKTICK + `h1{color:green;}` + BACKTICK + `}</style>`,
			},
		},
		{
			name:   "set:html on style with template literal attribute with variable",
			source: `<style set:html=` + BACKTICK + `${content}` + BACKTICK + ` />`,
			want: want{
				code: `<style>${` + BACKTICK + `${content}` + BACKTICK + `}</style>`,
			},
		},
		{
			name:   "set:html on Fragment",
			source: "<Fragment set:html={\"<p>&#x3C;i>This should NOT be italic&#x3C;/i></p>\"} />",
			want: want{
				code: "${$$renderComponent($$result,'Fragment',Fragment,{},{\"default\": () => $$render`${$$unescapeHTML(\"<p>&#x3C;i>This should NOT be italic&#x3C;/i></p>\")}`,})}",
			},
		},
		{
			name:   "set:html on Fragment with quoted attribute",
			source: "<Fragment set:html=\"<p>&#x3C;i>This should NOT be italic&#x3C;/i></p>\" />",
			want: want{
				code: "${$$renderComponent($$result,'Fragment',Fragment,{},{\"default\": () => $$render`${\"<p><i>This should NOT be italic</i></p>\"}`,})}",
			},
		},
		{
			name:   "set:html on Fragment with template literal attribute without variable",
			source: "<Fragment set:html=`<p>&#x3C;i>This should NOT be italic&#x3C;/i></p>` />",
			want: want{
				code: "${$$renderComponent($$result,'Fragment',Fragment,{},{\"default\": () => $$render`${`<p><i>This should NOT be italic</i></p>`}`,})}",
			},
		},
		{
			name:   "set:html on Fragment with template literal attribute with variable",
			source: `<Fragment set:html=` + BACKTICK + `${content}` + BACKTICK + ` />`,
			want: want{
				code: `${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `${` + BACKTICK + `${content}` + BACKTICK + `}` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "template literal attribute on component",
			source: `<Component class=` + BACKTICK + `red` + BACKTICK + ` />`,
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{"class":` + BACKTICK + `red` + BACKTICK + `})}`,
			},
		},
		{
			name:   "template literal attribute with variable on component",
			source: `<Component class=` + BACKTICK + `${color}` + BACKTICK + ` />`,
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{"class":` + BACKTICK + `${color}` + BACKTICK + `})}`,
			},
		},
		{
			name:   "define:vars on style",
			source: "<style>h1{color:green;}</style><style define:vars={{color:'green'}}>h1{color:var(--color)}</style><h1>testing</h1>",
			want: want{
				code:        `${$$maybeRenderHead($$result)}<h1 class="astro-vfs5oemv"${$$addAttribute($$definedVars, "style")}>testing</h1>`,
				definedVars: []string{"{color:'green'}"},
			},
		},
		{
			name:   "define:vars on style tag with style shorthand attribute on element",
			source: "<style define:vars={{color:'green'}}>h1{color:var(--color)}</style><h1 {style}>testing</h1>",
			want: want{
				code:        `${$$maybeRenderHead($$result)}<h1${$$addAttribute(` + BACKTICK + `${style}; ${$$definedVars}` + BACKTICK + `, "style")} class="astro-yiefzsdv">testing</h1>`,
				definedVars: []string{"{color:'green'}"},
			},
		},
		{
			name:   "define:vars on style tag with style expression attribute on element",
			source: "<style define:vars={{color:'green'}}>h1{color:var(--color)}</style><h1 style={myStyles}>testing</h1>",
			want: want{
				code:        `${$$maybeRenderHead($$result)}<h1${$$addAttribute(` + BACKTICK + `${myStyles}; ${$$definedVars}` + BACKTICK + `, "style")} class="astro-zwheddu6">testing</h1>`,
				definedVars: []string{"{color:'green'}"},
			},
		},
		{
			name:   "define:vars on style tag with style empty attribute on element",
			source: "<style define:vars={{color:'green'}}>h1{color:var(--color)}</style><h1 style>testing</h1>",
			want: want{
				code:        `${$$maybeRenderHead($$result)}<h1${$$addAttribute($$definedVars, "style")} class="astro-yvzw3g7h">testing</h1>`,
				definedVars: []string{"{color:'green'}"},
			},
		},
		{
			name:   "define:vars on style tag with style quoted attribute on element",
			source: "<style define:vars={{color:'green'}}>h1{color:var(--color)}</style><h1 style='color: yellow;'>testing</h1>",
			want: want{
				code:        `${$$maybeRenderHead($$result)}<h1${$$addAttribute(` + BACKTICK + `${"color: yellow;"}; ${$$definedVars}` + BACKTICK + `, "style")} class="astro-rrt5rq2h">testing</h1>`,
				definedVars: []string{"{color:'green'}"},
			},
		},
		{
			name:   "define:vars on style tag with style template literal attribute on element",
			source: "<style define:vars={{color:'green'}}>h1{color:var(--color)}</style><h1 style=`color: ${color};`>testing</h1>",
			want: want{
				code:        `${$$maybeRenderHead($$result)}<h1${$$addAttribute(` + BACKTICK + `${` + BACKTICK + `color: ${color};` + BACKTICK + `}; ${$$definedVars}` + BACKTICK + `, "style")} class="astro-33xvgaes">testing</h1>`,
				definedVars: []string{"{color:'green'}"},
			},
		},
		{
			name:   "multiple define:vars on style",
			source: "<style define:vars={{color:'green'}}>h1{color:var(--color)}</style><style define:vars={{color:'red'}}>h2{color:var(--color)}</style><h1>foo</h1><h2>bar</h2>",
			want: want{
				code:        `${$$maybeRenderHead($$result)}<h1 class="astro-6oxbqcst"${$$addAttribute($$definedVars, "style")}>foo</h1><h2 class="astro-6oxbqcst"${$$addAttribute($$definedVars, "style")}>bar</h2>`,
				definedVars: []string{"{color:'red'}", "{color:'green'}"},
			},
		},
		{
			name:   "define:vars on non-root elements",
			source: "<style define:vars={{color:'green'}}>h1{color:var(--color)}</style>{true ? <h1>foo</h1> : <h1>bar</h1>}",
			want: want{
				code:        `${true ? $$render` + BACKTICK + `${$$maybeRenderHead($$result)}<h1 class="astro-34ao5s3b"${$$addAttribute($$definedVars, "style")}>foo</h1>` + BACKTICK + ` : $$render` + BACKTICK + `<h1 class="astro-34ao5s3b"${$$addAttribute($$definedVars, "style")}>bar</h1>` + BACKTICK + `}`,
				definedVars: []string{"{color:'green'}"},
			},
		},
		{
			name:   "style is:inline define:vars",
			source: "<style is:inline define:vars={{color:'green'}}>h1{color:var(--color)}</style>",
			want: want{
				code: `<style>h1{color:var(--color)}</style>`,
			},
		},
		{
			name: "define:vars on script with StaticExpression turned on",
			// 1. An inline script with is:inline - right
			// 2. A hoisted script - wrong, shown up in scripts.add
			// 3. A define:vars hoisted script
			// 4. A define:vars inline script
			source: `<script is:inline>var one = 'one';</script><script>var two = 'two';</script><script define:vars={{foo:'bar'}}>var three = foo;</script><script is:inline define:vars={{foo:'bar'}}>var four = foo;</script>`,
			want: want{
				code: `<script>var one = 'one';</script><script>(function(){${$$defineScriptVars({foo:'bar'})}var three = foo;})();</script><script>(function(){${$$defineScriptVars({foo:'bar'})}var four = foo;})();</script>`,
				metadata: metadata{
					hoisted: []string{"{ type: 'inline', value: `var two = 'two';` }"},
				},
			},
		},
		{
			name: "define:vars on a module script with imports",
			// Should not wrap with { } scope.
			source: `<script type="module" define:vars={{foo:'bar'}}>import 'foo';\nvar three = foo;</script>`,
			want: want{
				code: `<script type="module">${$$defineScriptVars({foo:'bar'})}import 'foo';\\nvar three = foo;</script>`,
			},
		},
		{
			name:   "comments removed from attribute list",
			source: `<div><h1 {/* comment 1 */} value="1" {/* comment 2 */}>Hello</h1><Component {/* comment 1 */} value="1" {/* comment 2 */} /></div>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div><h1 value="1">Hello</h1>${$$renderComponent($$result,'Component',Component,{"value":"1",})}</div>`,
			},
		},
		{
			name:   "includes comments for shorthand attribute",
			source: `<div><h1 {/* comment 1 */ id /* comment 2 */}>Hello</h1><Component {/* comment 1 */ id /* comment 2 */}/></div>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div><h1${$$addAttribute(/* comment 1 */ id /* comment 2 */, "id")}>Hello</h1>${$$renderComponent($$result,'Component',Component,{"id":(/* comment 1 */ id /* comment 2 */)})}</div>`,
			},
		},
		{
			name:   "includes comments for expression attribute",
			source: `<div><h1 attr={/* comment 1 */ isTrue ? 1 : 2 /* comment 2 */}>Hello</h1><Component attr={/* comment 1 */ isTrue ? 1 : 2 /* comment 2 */}/></div>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div><h1${$$addAttribute(/* comment 1 */ isTrue ? 1 : 2 /* comment 2 */, "attr")}>Hello</h1>${$$renderComponent($$result,'Component',Component,{"attr":(/* comment 1 */ isTrue ? 1 : 2 /* comment 2 */)})}</div>`,
			},
		},
		{
			name:   "comment only expressions are removed I",
			source: `{/* a comment 1 */}<h1>{/* a comment 2*/}Hello</h1>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<h1>Hello</h1>`,
			},
		},
		{
			name: "comment only expressions are removed II",
			source: `{
    list.map((i) => (
        <Component>
            {
                // hello
            }
        </Component>
    ))
}`,
			want: want{
				code: `${
    list.map((i) => (
        $$render` + BACKTICK + `${$$renderComponent($$result,'Component',Component,{},{})}` + BACKTICK + `
    ))
}`,
			},
		},
		{
			name: "comment only expressions are removed III",
			source: `{
    list.map((i) => (
        <Component>
            {
                /* hello */
            }
        </Component>
    ))
}`,
			want: want{
				code: `${
    list.map((i) => (
        $$render` + BACKTICK + `${$$renderComponent($$result,'Component',Component,{},{})}` + BACKTICK + `
    ))
}`,
			},
		},
		{
			name:   "component with only a script",
			source: "<script>console.log('hello world');</script>",
			want: want{
				code:     ``,
				metadata: metadata{hoisted: []string{"{ type: 'inline', value: `console.log('hello world');` }"}},
			},
		},
		{
			name:     "passes filename into createComponent if passed into the compiler options",
			source:   `<div>test</div>`,
			filename: "/projects/app/src/pages/page.astro",
			want: want{
				code: `${$$maybeRenderHead($$result)}<div>test</div>`,
			},
		},
		{
			name:     "passes escaped filename into createComponent if it contains single quotes",
			source:   `<div>test</div>`,
			filename: "/projects/app/src/pages/page-with-'-quotes.astro",
			want: want{
				code: `${$$maybeRenderHead($$result)}<div>test</div>`,
			},
		},
		{
			name:     "maybeRenderHead not printed for hoisted scripts",
			source:   `<script></script><Layout></Layout>`,
			filename: "/projects/app/src/pages/page.astro",
			want: want{
				code: `${$$renderComponent($$result,'Layout',Layout,{})}`,
			},
		},
		{
			name:     "complex recursive component",
			source:   `{(<Fragment><Fragment set:html={` + BACKTICK + `<${Node.tag} ${stringifyAttributes(Node.attributes)}>` + BACKTICK + `} />{Node.children.map((child) => (<Astro.self node={child} />))}<Fragment set:html={` + BACKTICK + `</${Node.tag}>` + BACKTICK + `} /></Fragment>)}`,
			filename: "/projects/app/src/components/RenderNode.astro",
			want: want{
				code: `${($$render` + BACKTICK + `${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `${$$unescapeHTML(` + BACKTICK + `<${Node.tag} ${stringifyAttributes(Node.attributes)}>` + BACKTICK + `)}` + BACKTICK + `,})}${Node.children.map((child) => ($$render` + BACKTICK + `${$$renderComponent($$result,'Astro.self',Astro.self,{"node":(child)})}` + BACKTICK + `))}${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `${$$unescapeHTML(` + BACKTICK + `</${Node.tag}>` + BACKTICK + `)}` + BACKTICK + `,})}` + BACKTICK + `,})}` + BACKTICK + `)}`,
			},
		},
		{
			name:   "multibyte character + style",
			source: `<style>a { font-size: 16px; }</style><a class="test">ツ</a>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<a class="test astro-7vm74pjk">ツ</a>`,
			},
		},
		{
			name: "multibyte characters",
			source: `---
---
<h1>こんにちは</h1>`,
			want: want{
				code: `${$$maybeRenderHead($$result)}<h1>こんにちは</h1>`,
			},
		},

		{
			name:   "multibyte character + script",
			source: `<script>console.log('foo')</script><a class="test">ツ</a>`,
			want: want{
				code:     `${$$maybeRenderHead($$result)}<a class="test">ツ</a>`,
				metadata: metadata{hoisted: []string{fmt.Sprintf(`{ type: 'inline', value: %sconsole.log('foo')%s }`, BACKTICK, BACKTICK)}},
			},
		},

		{
			name:        "transition:name with an expression",
			source:      `<div transition:name={one + '-' + 'two'}></div>`,
			filename:    "/projects/app/src/pages/page.astro",
			transitions: true,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div${$$addAttribute($$renderTransition($$result, "daiq24ry", "", (one + '-' + 'two')), "data-astro-transition-scope")}></div>`,
			},
		},
		{
			name:        "transition:name with an template literal",
			source:      "<div transition:name=`${one}-two`></div>",
			filename:    "/projects/app/src/pages/page.astro",
			transitions: true,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div${$$addAttribute($$renderTransition($$result, "vvov4lyr", "", ` + BACKTICK + `${one}-two` + BACKTICK + `), "data-astro-transition-scope")}></div>`,
			},
		},
		{
			name:        "transition:animate with an expression",
			source:      "<div transition:animate={slide({duration:15})}></div>",
			filename:    "/projects/app/src/pages/page.astro",
			transitions: true,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div${$$addAttribute($$renderTransition($$result, "ih7yuffh", (slide({duration:15})), ""), "data-astro-transition-scope")}></div>`,
			},
		},
		{
			name:        "transition:animate on Component",
			source:      `<Component class="bar" transition:animate="morph"></Component>`,
			filename:    "/projects/app/src/pages/page.astro",
			transitions: true,
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{"class":"bar","data-astro-transition-scope":($$renderTransition($$result, "wkm5vset", "morph", ""))})}`,
			},
		},
		{
			name:        "transition:persist converted to a data attribute",
			source:      `<div transition:persist></div>`,
			transitions: true,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div${$$addAttribute($$createTransitionScope($$result, "pflz5ime"), "data-astro-transition-persist")}></div>`,
			},
		},
		{
			name:        "transition:persist uses transition:name if defined",
			source:      `<div transition:persist transition:name="foo"></div>`,
			transitions: true,
			want: want{
				code: `${$$maybeRenderHead($$result)}<div data-astro-transition-persist="foo"${$$addAttribute($$renderTransition($$result, "peuy4xf7", "", "foo"), "data-astro-transition-scope")}></div>`,
			},
		},
		{
			name:        "transition:persist-props converted to a data attribute",
			source:      `<my-island transition:persist transition:persist-props="false"></my-island>`,
			transitions: true,
			want: want{
				code: `${$$renderComponent($$result,'my-island','my-island',{"data-astro-transition-persist-props":"false","data-astro-transition-persist":($$createTransitionScope($$result, "otghnj5u"))})}`,
			},
		},
		{
			name:   "trailing expression",
			source: `<Component />{}`,
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{})}${(void 0)}`,
			},
		},
		{
			name: "nested head content stays in the head",
			source: `---
const meta = { title: 'My App' };
---

<html>
	<head>
		<meta charset="utf-8" />

		{
			meta && <title>{meta.title}</title>
		}

		<meta name="after">
	</head>
	<body>
		<h1>My App</h1>
	</body>
</html>`,
			want: want{
				frontmatter: []string{"", `const meta = { title: 'My App' };`},
				code:        `<html>	<head>		<meta charset="utf-8">		${			meta && $$render` + BACKTICK + `<title>${meta.title}</title>` + BACKTICK + `		}		<meta name="after">	${$$renderHead($$result)}</head>	<body>		<h1>My App</h1>	</body></html>`,
			},
		},
	}

	for _, tt := range tests {
		if tt.only {
			tests = make([]testcase, 0)
			tests = append(tests, tt)
			break
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// transform output from source
			code := test_utils.Dedent(tt.source)

			doc, err := astro.Parse(strings.NewReader(code))
			h := handler.NewHandler(code, "<stdin>")

			if err != nil {
				t.Error(err)
			}

			hash := astro.HashString(code)
			transform.ExtractStyles(doc)
			// combine from tt.transformOptions
			transformOptions := transform.TransformOptions{
				Scope:        hash,
				RenderScript: tt.transformOptions.RenderScript,
			}
			transform.Transform(doc, transformOptions, h) // note: we want to test Transform in context here, but more advanced cases could be tested separately

			result := PrintToJS(code, doc, 0, transform.TransformOptions{
				Scope:                   "XXXX",
				InternalURL:             "http://localhost:3000/",
				Filename:                tt.filename,
				AstroGlobalArgs:         "'https://astro.build'",
				TransitionsAnimationURL: "transitions.css",
			}, h)
			output := string(result.Output)

			toMatch := INTERNAL_IMPORTS
			if strings.Count(tt.source, "transition:") > 0 {
				toMatch += `import "transitions.css";`
			}
			if len(tt.want.frontmatter) > 0 {
				toMatch += test_utils.Dedent(tt.want.frontmatter[0])
			}
			moduleSpecRe := regexp.MustCompile(`specifier:\s*('[^']+'),\s*assert:\s*([^}]+\})`)
			if len(tt.want.metadata.modules) > 0 {
				toMatch += "\n\n"
				for i, m := range tt.want.metadata.modules {
					spec := moduleSpecRe.FindSubmatch([]byte(m)) // 0: full match, 1: submatch
					asrt := ""
					if string(spec[2]) != "{}" {
						asrt = " assert " + string(spec[2])
					}
					toMatch += fmt.Sprintf("import * as $$module%s from %s%s;\n", strconv.Itoa(i+1), string(spec[1]), asrt)
				}
			}
			// build metadata object from provided strings
			metadata := "{ "
			// metadata.modules
			metadata += "modules: ["
			if len(tt.want.metadata.modules) > 0 {
				for i, m := range tt.want.metadata.modules {
					if i > 0 {
						metadata += ", "
					}
					metadata += m
				}
			}
			metadata += "]"
			// metadata.hydratedComponents
			metadata += ", hydratedComponents: ["
			if len(tt.want.metadata.hydratedComponents) > 0 {
				for i, c := range tt.want.hydratedComponents {
					if i > 0 {
						metadata += ", "
					}
					metadata += c
				}
			}
			metadata += "]"
			// metadata.clientOnlyComponents
			metadata += ", clientOnlyComponents: ["
			if len(tt.want.metadata.clientOnlyComponents) > 0 {
				for i, c := range tt.want.clientOnlyComponents {
					if i > 0 {
						metadata += ", "
					}
					metadata += fmt.Sprintf("'%s'", c)
				}
			}
			metadata += "]"
			// directives
			metadata += ", hydrationDirectives: new Set(["
			if len(tt.want.hydrationDirectives) > 0 {
				for i, c := range tt.want.hydrationDirectives {
					if i > 0 {
						metadata += ", "
					}
					metadata += fmt.Sprintf("'%s'", c)
				}
			}
			metadata += "])"
			// metadata.hoisted
			metadata += ", hoisted: ["
			if len(tt.want.metadata.hoisted) > 0 {
				for i, h := range tt.want.hoisted {
					if i > 0 {
						metadata += ", "
					}
					metadata += h
				}
			}
			metadata += "] }"

			patharg := "import.meta.url"
			if tt.filename != "" {
				escapedFilename := strings.ReplaceAll(tt.filename, "'", "\\'")
				patharg = fmt.Sprintf("\"%s\"", escapedFilename)
			}
			toMatch += "\n\n" + fmt.Sprintf("export const %s = %s(%s, %s);\n\n", METADATA, CREATE_METADATA, patharg, metadata)
			toMatch += test_utils.Dedent(CREATE_ASTRO_CALL) + "\n"
			if len(tt.want.getStaticPaths) > 0 {
				toMatch += strings.TrimSpace(test_utils.Dedent(tt.want.getStaticPaths)) + "\n\n"
			}
			toMatch += test_utils.Dedent(PRELUDE) + "\n"
			if len(tt.want.frontmatter) > 1 {
				toMatch += strings.TrimSpace(test_utils.Dedent(tt.want.frontmatter[1]))
			}
			toMatch += "\n"
			if len(tt.want.definedVars) > 0 {
				toMatch = toMatch + "const $$definedVars = $$defineStyleVars(["
				for i, d := range tt.want.definedVars {
					if i > 0 {
						toMatch += ","
					}
					toMatch += d
				}
				toMatch += "]);\n"
			}
			// code
			toMatch += test_utils.Dedent(fmt.Sprintf("%s%s", RETURN, tt.want.code))
			// HACK: add period to end of test to indicate significant preceding whitespace (otherwise stripped by dedent)
			if strings.HasSuffix(toMatch, ".") {
				toMatch = strings.TrimRight(toMatch, ".")
			}

			if len(tt.filename) > 0 {
				escapedFilename := strings.ReplaceAll(tt.filename, "'", "\\'")
				toMatch += suffixWithFilename(escapedFilename, tt.transitions)
				toMatch = strings.Replace(toMatch, "$$Component", getComponentName(tt.filename), -1)
			} else if tt.transitions {
				toMatch += SUFFIX_EXP_TRANSITIONS
			} else {
				toMatch += SUFFIX
			}

			// compare to expected string, show diff if mismatch
			if diff := test_utils.ANSIDiff(test_utils.RemoveNewlines(test_utils.Dedent(toMatch)), test_utils.RemoveNewlines(test_utils.Dedent(output))); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPrintToJSON(t *testing.T) {
	tests := []jsonTestcase{
		{
			name:   "basic",
			source: `<h1>Hello world!</h1>`,
			want:   []ASTNode{{Type: "element", Name: "h1", Children: []ASTNode{{Type: "text", Value: "Hello world!"}}}},
		},
		{
			name:   "expression",
			source: `<h1>Hello {world}</h1>`,
			want:   []ASTNode{{Type: "element", Name: "h1", Children: []ASTNode{{Type: "text", Value: "Hello "}, {Type: "expression", Children: []ASTNode{{Type: "text", Value: "world"}}}}}},
		},
		{
			name:   "Component",
			source: `<Component />`,
			want:   []ASTNode{{Type: "component", Name: "Component"}},
		},
		{
			name:   "custom-element",
			source: `<custom-element />`,
			want:   []ASTNode{{Type: "custom-element", Name: "custom-element"}},
		},
		{
			name:   "Doctype",
			source: `<!DOCTYPE html />`,
			want:   []ASTNode{{Type: "doctype", Value: "html"}},
		},
		{
			name:   "Comment",
			source: `<!--hello-->`,
			want:   []ASTNode{{Type: "comment", Value: "hello"}},
		},
		{
			name:   "Comment preserves whitespace",
			source: `<!-- hello -->`,
			want:   []ASTNode{{Type: "comment", Value: " hello "}},
		},
		{
			name:   "Fragment Shorthand",
			source: `<>Hello</>`,
			want:   []ASTNode{{Type: "fragment", Name: "", Children: []ASTNode{{Type: "text", Value: "Hello"}}}},
		},
		{
			name:   "Fragment Literal",
			source: `<Fragment>World</Fragment>`,
			want:   []ASTNode{{Type: "fragment", Name: "Fragment", Children: []ASTNode{{Type: "text", Value: "World"}}}},
		},
		{
			name: "Frontmatter",
			source: `---
const a = "hey"
---
<div>{a}</div>`,
			want: []ASTNode{{Type: "frontmatter", Value: "\nconst a = \"hey\"\n"}, {Type: "element", Name: "div", Children: []ASTNode{{Type: "expression", Children: []ASTNode{{Type: "text", Value: "a"}}}}}},
		},
		{
			name: "JSON escape",
			source: `---
const a = "\n"
const b = "\""
const c = '\''
---
{a + b + c}`,
			want: []ASTNode{{Type: "frontmatter", Value: "\nconst a = \"\\n\"\nconst b = \"\\\"\"\nconst c = '\\''\n"}, {Type: "expression", Children: []ASTNode{{Type: "text", Value: "a + b + c"}}}},
		},
		{
			name:   "Preserve namespaces",
			source: `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><rect xlink:href="#id"></svg>`,
			want:   []ASTNode{{Type: "element", Name: "svg", Attributes: []ASTNode{{Type: "attribute", Kind: "quoted", Name: "xmlns", Value: "http://www.w3.org/2000/svg", Raw: `"http://www.w3.org/2000/svg"`}, {Type: "attribute", Kind: "quoted", Name: "xmlns:xlink", Value: "http://www.w3.org/1999/xlink", Raw: `"http://www.w3.org/1999/xlink"`}}, Children: []ASTNode{{Type: "element", Name: "rect", Attributes: []ASTNode{{Type: "attribute", Kind: "quoted", Name: "xlink:href", Value: "#id", Raw: `"#id"`}}}}}},
		},
		{
			name:   "style before html",
			source: `<style></style><html><body><h1>Hello world!</h1></body></html>`,
			want:   []ASTNode{{Type: "element", Name: "style"}, {Type: "element", Name: "html", Children: []ASTNode{{Type: "element", Name: "body", Children: []ASTNode{{Type: "element", Name: "h1", Children: []ASTNode{{Type: "text", Value: "Hello world!"}}}}}}}},
		},
		{
			name:   "style after html",
			source: `<html><body><h1>Hello world!</h1></body></html><style></style>`,
			want:   []ASTNode{{Type: "element", Name: "html", Children: []ASTNode{{Type: "element", Name: "body", Children: []ASTNode{{Type: "element", Name: "h1", Children: []ASTNode{{Type: "text", Value: "Hello world!"}}}}}}}, {Type: "element", Name: "style"}},
		},
		{
			name:   "style in html",
			source: `<html><body><h1>Hello world!</h1></body><style></style></html>`,
			want:   []ASTNode{{Type: "element", Name: "html", Children: []ASTNode{{Type: "element", Name: "body", Children: []ASTNode{{Type: "element", Name: "h1", Children: []ASTNode{{Type: "text", Value: "Hello world!"}}}}}, {Type: "element", Name: "style"}}}},
		},
		{
			name:   "style in body",
			source: `<html><body><h1>Hello world!</h1><style></style></body></html>`,
			want:   []ASTNode{{Type: "element", Name: "html", Children: []ASTNode{{Type: "element", Name: "body", Children: []ASTNode{{Type: "element", Name: "h1", Children: []ASTNode{{Type: "text", Value: "Hello world!"}}}, {Type: "element", Name: "style"}}}}}},
		},
		{
			name:   "element with unterminated double quote attribute",
			source: `<main id="gotcha />`,
			want:   []ASTNode{{Type: "element", Name: "main", Attributes: []ASTNode{{Type: "attribute", Kind: "quoted", Name: "id", Value: "gotcha", Raw: "\"gotcha"}}}},
		},
		{
			name:   "element with unterminated single quote attribute",
			source: `<main id='gotcha />`,
			want:   []ASTNode{{Type: "element", Name: "main", Attributes: []ASTNode{{Type: "attribute", Kind: "quoted", Name: "id", Value: "gotcha", Raw: "'gotcha"}}}},
		},
		{
			name:   "element with unterminated template literal attribute",
			source: `<main id=` + BACKTICK + `gotcha />`,
			want:   []ASTNode{{Type: "element", Name: "main", Attributes: []ASTNode{{Type: "attribute", Kind: "template-literal", Name: "id", Value: "gotcha", Raw: "`gotcha"}}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// transform output from source
			code := test_utils.Dedent(tt.source)

			doc, err := astro.ParseWithOptions(strings.NewReader(code), astro.ParseOptionEnableLiteral(true), astro.ParseOptionWithHandler(&handler.Handler{}))

			if err != nil {
				t.Error(err)
			}

			root := ASTNode{Type: "root", Children: tt.want}
			toMatch := root.String()

			result := PrintToJSON(code, doc, types.ParseOptions{Position: false})

			if diff := test_utils.ANSIDiff(test_utils.Dedent(string(toMatch)), test_utils.Dedent(string(result.Output))); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
