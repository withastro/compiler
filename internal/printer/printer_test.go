package printer

import (
	"fmt"
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
var PRELUDE = fmt.Sprintf(`const $$Component = %s(($$result, $$props, %s) => {`, CREATE_COMPONENT, SLOTS)
var PRELUDE_WITH_ASYNC = fmt.Sprintf(`const $$Component = %s(async ($$result, $$props, %s) => {`, CREATE_COMPONENT, SLOTS)
var PRELUDE_ASTRO_GLOBAL = fmt.Sprintf(`const Astro = $$result.createAstro($$Astro, $$props, %s);
Astro.self = $$Component;`, SLOTS)
var RETURN = fmt.Sprintf("return %s%s", TEMPLATE_TAG, BACKTICK)
var SUFFIX = fmt.Sprintf("%s;", BACKTICK) + `
}, undefined, undefined);
export default $$Component;`
var SUFFIX_EXP_TRANSITIONS = fmt.Sprintf("%s;", BACKTICK) + `
}, undefined, 'self');
export default $$Component;`
var CREATE_ASTRO_CALL = "const $$Astro = $$createAstro('https://astro.build');\nconst Astro = $$Astro;"
var RENDER_HEAD_RESULT = "${$$renderHead($$result)}"

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
}

type jsonTestcase struct {
	name   string
	source string
	only   bool
}

func TestPrinter(t *testing.T) {
	longRandomString := ""
	for i := 0; i < 40; i++ {
		longRandomString += "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[];:'\",.?"
	}

	tests := []testcase{
		{
			name:   "text only",
			source: `Foo`,
		},
		{
			name:   "unusual line terminator I",
			source: `Pre-set & Time-limited \u2028holiday campaigns`,
		},
		{
			name:   "unusual line terminator II",
			source: `Pre-set & Time-limited  holiday campaigns`,
		},
		{
			name:   "basic (no frontmatter)",
			source: `<button>Click</button>`,
		},
		{
			name:   "basic renderHead",
			source: `<html><head><title>Ah</title></head></html>`,
		},
		{
			name:   "head inside slot",
			source: `<html><slot><head></head></slot></html>`,
		},
		{
			name:   "head slot",
			source: `<html><head><slot /></html>`,
		},
		{
			name:   "head slot II",
			source: `<html><head><slot /></head><body class="a"></body></html>`,
		},
		{
			name:   "head slot III",
			source: `<html><head><slot name="baseHeadExtension"><meta property="test2" content="test2"/></slot></head>`,
		},
		{
			name:   "ternary component",
			source: `{special ? <ChildDiv><p>Special</p></ChildDiv> : <p>Not special</p>}`,
		},
		{
			name:   "ternary layout",
			source: `{toggleError ? <BaseLayout><h1>SITE: {Astro.site}</h1></BaseLayout> : <><h1>SITE: {Astro.site}</h1></>}`,
		},
		{
			name:   "orphan slot",
			source: `<slot />`,
		},
		{
			name:   "conditional slot",
			source: `<Component>{value && <div slot="test">foo</div>}</Component>`,
		},
		{
			name:   "ternary slot",
			source: `<Component>{Math.random() > 0.5 ? <div slot="a">A</div> : <div slot="b">B</div>}</Component>`,
		},
		{
			name:   "function expression slots I",
			source: "<Component>\n{() => { switch (value) {\ncase 'a': return <div slot=\"a\">A</div>\ncase 'b': return <div slot=\"b\">B</div>\ncase 'c': return <div slot=\"c\">C</div>\n}\n}}\n</Component>",
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
		},
		{
			name:   "expression slot",
			source: `<Component>{true && <div slot="a">A</div>}{false && <div slot="b">B</div>}</Component>`,
		},
		{
			name:   "preserve is:inline slot",
			source: `<slot is:inline />`,
		},
		{
			name:   "preserve is:inline slot II",
			source: `<slot name="test" is:inline />`,
		},
		{
			name:   "slot with fallback",
			source: `<body><slot><p>Hello world!</p></slot><body>`,
		},
		{
			name:   "slot with fallback II",
			source: `<slot name="test"><p>Hello world!</p></slot>`,
		},
		{
			name:   "slot with fallback III",
			source: `<div><slot name="test"><p>Fallback</p></slot></div>`,
		},
		{
			name: "Preserve slot whitespace",
			source: `<Component>
  <p>Paragraph 1</p>
  <p>Paragraph 2</p>
</Component>`,
		},
		{
			name:   "text only",
			source: "Hello!",
		},
		{
			name:   "custom-element",
			source: "{show && <client-only-element></client-only-element>}",
		},
		{
			name:   "attribute with template literal",
			source: "<a :href=\"`/home`\">Home</a>",
		},
		{
			name:   "attribute with template literal interpolation",
			source: "<a :href=\"`/${url}`\">Home</a>",
		},
		{
			name: "basic (frontmatter)",
			source: `---
const href = '/about';
---
<a href={href}>About</a>`,
		},
		{
			name: "getStaticPaths (basic)",
			source: `---
export const getStaticPaths = async () => {
	return { paths: [] }
}
---
<div></div>`,
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
		},
		{
			name: "export member does not panic",
			source: `---
mod.export();
---
<div />`,
		},
		{
			name: "export comments I",
			source: `---
// hmm
export const foo = 0
/*
*/
---`,
		},
		{
			name: "export comments II",
			source: `---
// hmm
export const foo = 0;
/*
*/
---`,
		},
		{
			name: "import assertions",
			source: `---
import data from "test" assert { type: 'json' };
---
`,
		},
		{
			name: "import to identifier named assert",

			source: `---
import assert from 'test';
---`,
		},
		{
			name:   "no expressions in math",
			source: `<p>Hello, world! This is a <em>buggy</em> formula: <span class="math math-inline"><span class="katex"><span class="katex-mathml"><math xmlns="http://www.w3.org/1998/Math/MathML"><semantics><mrow><mi>f</mi><mspace></mspace><mspace width="0.1111em"></mspace><mo lspace="0em" rspace="0.17em"></mo><mtext> ⁣</mtext><mo lspace="0em" rspace="0em">:</mo><mspace width="0.3333em"></mspace><mi>X</mi><mo>→</mo><msup><mi mathvariant="double-struck">R</mi><mrow><mn>2</mn><mi>x</mi></mrow></msup></mrow><annotation encoding="application/x-tex">f\colon X \to \mathbb R^{2x}</annotation></semantics></math></span><span class="katex-html" aria-hidden="true"><span class="base"><span class="strut" style="height:0.8889em;vertical-align:-0.1944em;"></span><span class="mord mathnormal" style="margin-right:0.10764em;">f</span><span class="mspace nobreak"></span><span class="mspace" style="margin-right:0.1111em;"></span><span class="mpunct"></span><span class="mspace" style="margin-right:-0.1667em;"></span><span class="mspace" style="margin-right:0.1667em;"></span><span class="mord"><span class="mrel">:</span></span><span class="mspace" style="margin-right:0.3333em;"></span><span class="mord mathnormal" style="margin-right:0.07847em;">X</span><span class="mspace" style="margin-right:0.2778em;"></span><span class="mrel">→</span><span class="mspace" style="margin-right:0.2778em;"></span></span><span class="base"><span class="strut" style="height:0.8141em;"></span><span class="mord"><span class="mord mathbb">R</span><span class="msupsub"><span class="vlist-t"><span class="vlist-r"><span class="vlist" style="height:0.8141em;"><span style="top:-3.063em;margin-right:0.05em;"><span class="pstrut" style="height:2.7em;"></span><span class="sizing reset-size6 size3 mtight"><span class="mord mtight"><span class="mord mtight">2</span><span class="mord mathnormal mtight">x</span></span></span></span></span></span></span></span></span></span></span></span></span></p>`,
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
		},
		{
			name: "type import",
			source: `---
import type data from "test"
---

<div>{data}</div>
`,
		},
		{
			name:   "no expressions in math",
			source: `<p>Hello, world! This is a <em>buggy</em> formula: <span class="math math-inline"><span class="katex"><span class="katex-mathml"><math xmlns="http://www.w3.org/1998/Math/MathML"><semantics><mrow><mi>f</mi><mspace></mspace><mspace width="0.1111em"></mspace><mo lspace="0em" rspace="0.17em"></mo><mtext> ⁣</mtext><mo lspace="0em" rspace="0em">:</mo><mspace width="0.3333em"></mspace><mi>X</mi><mo>→</mo><msup><mi mathvariant="double-struck">R</mi><mrow><mn>2</mn><mi>x</mi></mrow></msup></mrow><annotation encoding="application/x-tex">f\colon X \to \mathbb R^{2x}</annotation></semantics></math></span><span class="katex-html" aria-hidden="true"><span class="base"><span class="strut" style="height:0.8889em;vertical-align:-0.1944em;"></span><span class="mord mathnormal" style="margin-right:0.10764em;">f</span><span class="mspace nobreak"></span><span class="mspace" style="margin-right:0.1111em;"></span><span class="mpunct"></span><span class="mspace" style="margin-right:-0.1667em;"></span><span class="mspace" style="margin-right:0.1667em;"></span><span class="mord"><span class="mrel">:</span></span><span class="mspace" style="margin-right:0.3333em;"></span><span class="mord mathnormal" style="margin-right:0.07847em;">X</span><span class="mspace" style="margin-right:0.2778em;"></span><span class="mrel">→</span><span class="mspace" style="margin-right:0.2778em;"></span></span><span class="base"><span class="strut" style="height:0.8141em;"></span><span class="mord"><span class="mord mathbb">R</span><span class="msupsub"><span class="vlist-t"><span class="vlist-r"><span class="vlist" style="height:0.8141em;"><span style="top:-3.063em;margin-right:0.05em;"><span class="pstrut" style="height:2.7em;"></span><span class="sizing reset-size6 size3 mtight"><span class="mord mtight"><span class="mord mtight">2</span><span class="mord mathnormal mtight">x</span></span></span></span></span></span></span></span></span></span></span></span></span></p>`,
		},

		{
			name: "css imports are not included in module metadata",
			source: `---
			import './styles.css';
			---
			`,
		},
		{
			name:   "solidus in template literal expression",
			source: "<div value={`${attr ? `a/b` : \"c\"} awesome`} />",
		},
		{
			name:   "nested template literal expression",
			source: "<div value={`${attr ? `a/b ${`c`}` : \"d\"} awesome`} />",
		},
		{
			name:   "component in expression with its child expression before its child element",
			source: "{list.map(() => (<Component>{name}<link rel=\"stylesheet\" /></Component>))}",
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
		},
		{
			name: "nested template literal expression",
			source: `<html lang="en">
<body>
{Object.keys(importedAuthors).map(author => <p><div>hello</div></p>)}
{Object.keys(importedAuthors).map(author => <p><div>{author}</div></p>)}
</body>
</html>`,
		},
		{
			name:   "complex nested template literal expression",
			source: "<div value={`${attr ? `a/b ${`c ${`d ${cool}`}`}` : \"d\"} ahhhh`} />",
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
		},
		{
			name:   "component with quoted attributes",
			source: `<Component is='"cool"' />`,
		},
		{
			name:   "slot with quoted attributes",
			source: `<Component><div slot='"name"' /></Component>`,
		},
		{
			name:   "#955 ternary slot with text",
			source: `<Component>Hello{isLeaf ? <p>Leaf</p> : <p>Branch</p>}world</Component>`,
		},
		{
			name:   "#955 ternary slot with elements",
			source: `<Component><div>{isLeaf ? <p>Leaf</p> : <p>Branch</p>}</div></Component>`,
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
		},
		{
			name:   "noscript styles",
			source: `<noscript><style>div { color: red; }</style></noscript>`,
		},
		{
			name:   "noscript deep styles",
			source: `<body><noscript><div><div><div><style>div { color: red; }</style></div></div></div></noscript></body>`,
		},
		{
			name:   "noscript only",
			source: `<noscript><h1>Hello world</h1></noscript>`,
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
		},
		{
			name:   "iframe",
			source: `<iframe src="something" />`,
		},
		{
			name:   "conditional render",
			source: `<body>{false ? <div>#f</div> : <div>#t</div>}</body>`,
		},
		{
			name:   "conditional noscript",
			source: `{mode === "production" && <noscript>Hello</noscript>}`,
		},
		{
			name:   "conditional iframe",
			source: `{bool && <iframe src="something">content</iframe>}`,
		},
		{
			name:   "simple ternary",
			source: `<body>{link ? <a href="/">{link}</a> : <div>no link</div>}</body>`,
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
		},
		{
			name:   "map without component",
			source: `<header><nav>{menu.map((item) => <a href={item.href}>{item.title}</a>)}</nav></header>`,
		},
		{
			name:   "map with component",
			source: `<header><nav>{menu.map((item) => <a href={item.href}>{item.title}</a>)}</nav><Hello/></header>`,
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
		},
		{
			name:   "backtick in HTML comment",
			source: "<body><!-- `npm install astro` --></body>",
		},
		{
			name:   "HTML comment in component inside expression I",
			source: "{(() => <Component><!--Hi--></Component>)}",
		},
		{
			name:   "HTML comment in component inside expression II",
			source: "{list.map(() => <Component><!--Hi--></Component>)}",
		},
		{
			name:   "nested expressions",
			source: `<article>{(previous || next) && <aside>{previous && <div>Previous Article: <a rel="prev" href={new URL(previous.link, Astro.site).pathname}>{previous.text}</a></div>}{next && <div>Next Article: <a rel="next" href={new URL(next.link, Astro.site).pathname}>{next.text}</a></div>}</aside>}</article>`,
		},
		{
			name:   "nested expressions II",
			source: `<article>{(previous || next) && <aside>{previous && <div>Previous Article: <a rel="prev" href={new URL(previous.link, Astro.site).pathname}>{previous.text}</a></div>} {next && <div>Next Article: <a rel="next" href={new URL(next.link, Astro.site).pathname}>{next.text}</a></div>}</aside>}</article>`,
		},
		{
			name:   "nested expressions III",
			source: `<div>{x.map((x) => x ? <div>{true ? <span>{x}</span> : null}</div> : <div>{false ? null : <span>{x}</span>}</div>)}</div>`,
		},
		{
			name:   "nested expressions IV",
			source: `<div>{() => { if (value > 0.25) { return <span>Default</span> } else if (value > 0.5) { return <span>Another</span> } else if (value > 0.75) { return <span>Other</span> } return <span>Yet Other</span> }}</div>`,
		},
		{
			name:   "nested expressions V",
			source: `<div><h1>title</h1>{list.map(group => <Fragment><h2>{group.label}</h2>{group.items.map(item => <span>{item}</span>)}</Fragment>)}</div>`,
		},
		{
			name:   "nested expressions VI",
			source: `<div>{()=>{ if (true) { return <hr />;} if (true) { return <img />;}}}</div>`,
		},
		{
			name:   "nested expressions VII",
			source: `<div>{() => { if (value > 0.25) { return <br />;} else if (value > 0.5) { return <hr />;} else if (value > 0.75) { return <div />;} return <div>Yaaay</div>;}</div>`,
		},
		{
			name:   "nested expressions VIII",
			source: `<div>{ items.map(({ type, ...data }) => { switch (type) { case 'card': { return ( <Card {...data} /> ); } case 'paragraph': { return ( <p>{data.body}</p>);}}})}</div>`,
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
		},
		{
			name: "slots (expression)",
			source: `
<Component {data}>
	{items.map(item => <div>{item}</div>)}
</Component>`,
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
		},
		{
			name: "script hoist with frontmatter",
			source: `---
---
<script type="module" hoist>console.log("Hello");</script>`,
		},
		{
			name: "script hoist without frontmatter",
			source: `
							<main>
								<script type="module" hoist>console.log("Hello");</script>
							`,
		},
		{
			name:   "scriptinline",
			source: `<main><script is:inline type="module">console.log("Hello");</script>`,
		},
		{
			name:   "script define:vars I",
			source: `<script define:vars={{ value: 0 }}>console.log(value);</script>`,
		},
		{
			name:   "script define:vars II",
			source: `<script define:vars={{ "dash-case": true }}>console.log(dashCase);</script>`,
		},
		{
			name:   "script before elements",
			source: `<script>Here</script><div></div>`,
		},
		{
			name:   "script (renderScript: true)",
			source: `<main><script>console.log("Hello");</script>`,
			transformOptions: transform.TransformOptions{
				RenderScript: true,
			},
			filename: "/src/pages/index.astro",
		},
		{
			name:   "script multiple (renderScript: true)",
			source: `<main><script>console.log("Hello");</script><script>console.log("World");</script>`,
			transformOptions: transform.TransformOptions{
				RenderScript: true,
			},
			filename: "/src/pages/index.astro",
		},
		{
			name:   "script external (renderScript: true)",
			source: `<main><script src="./hello.js"></script>`,
			transformOptions: transform.TransformOptions{
				RenderScript: true,
			},
			filename: "/src/pages/index.astro",
		},
		{
			name:   "script external in expression (renderScript: true)",
			source: `<main>{<script src="./hello.js"></script>}`,
			transformOptions: transform.TransformOptions{
				RenderScript: true,
			},
			filename: "/src/pages/index.astro",
		},
		{
			// maintain the original behavior, though it may be
			// unneeded as renderScript is now on by default
			name:   "script external in expression (renderScript: false)",
			source: `<main>{<script src="./hello.js"></script>}`,
			filename: "/src/pages/index.astro",
		},
		{
			name:   "script in expression (renderScript: true)",
			source: `<main>{true && <script>console.log("hello")</script>}`,
			transformOptions: transform.TransformOptions{
				RenderScript: true,
			},
			filename: "/src/pages/index.astro",
		},
		{
			name:     "script in expression (renderScript: false)",
			source:   `<main>{false && <script>console.log("hello")</script>}`,
			filename: "/src/pages/index.astro",
		},
		{
			name:   "script inline (renderScript: true)",
			source: `<main><script is:inline type="module">console.log("Hello");</script>`,
			transformOptions: transform.TransformOptions{
				RenderScript: true,
			},
		},
		{
			name:   "script mixed handled and inline (renderScript: true)",
			source: `<main><script>console.log("Hello");</script><script is:inline>console.log("World");</script>`,
			transformOptions: transform.TransformOptions{
				RenderScript: true,
			},
			filename: "/src/pages/index.astro",
		},
		{
			name:   "text after title expression",
			source: `<title>a {expr} b</title>`,
		},
		{
			name:   "text after title expressions",
			source: `<title>a {expr} b {expr} c</title>`,
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
		},
		{
			name: "slots (named only)",
			source: `<Slotted>
      <span slot="a">A</span>
      <span slot="b">B</span>
      <span slot="c">C</span>
    </Slotted>`,
		},
		{
			name:   "condition expressions at the top-level",
			source: `{cond && <span></span>}{cond && <strong></strong>}`,
		},
		{
			name:   "condition expressions at the top-level with head content",
			source: `{cond && <meta charset=utf8>}{cond && <title>My title</title>}`,
		},
		{
			name: "custom elements",
			source: `---
import 'test';
---
<my-element></my-element>`,
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
		},
		{
			name:   "Component siblings are siblings",
			source: `<BaseHead></BaseHead><link href="test">`,
		},
		{
			name:   "Self-closing components siblings are siblings",
			source: `<BaseHead /><link href="test">`,
		},
		{
			name:   "Self-closing script in head works",
			source: `<html><head><script is:inline /></head><html>`,
		},
		{
			name:   "Self-closing title",
			source: `<title />`,
		},
		{
			name:   "Self-closing title II",
			source: `<html><head><title /></head><body></body></html>`,
		},
		{
			name:   "Self-closing components in head can have siblings",
			source: `<html><head><BaseHead /><link href="test"></head><html>`,
		},
		{
			name:   "Self-closing formatting elements",
			source: `<div id="1"><div id="2"><div id="3"><i/><i/><i/></div></div></div>`,
		},
		{
			name: "Self-closing formatting elements 2",
			source: `<body>
  <div id="1"><div id="2"><div id="3"><i id="a" /></div></div></div>
  <div id="4"><div id="5"><div id="6"><i id="b" /></div></div></div>
  <div id="7"><div id="8"><div id="9"><i id="c" /></div></div></div>
</body>`,
		},
		{
			name: "Nested HTML in expressions, wrapped in parens",
			source: `---
const image = './penguin.png';
const canonicalURL = new URL('http://example.com');
---
{image && (<meta property="og:image" content={new URL(image, canonicalURL)}>)}`,
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
		},
		{
			name: "dynamic import",
			source: `---
const markdownDocs = await Astro.glob('../markdown/*.md')
const article2 = await import('../markdown/article2.md')
---
<div />
`,
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
		},
		{
			name:   "SVG styles",
			source: `<svg><style>path { fill: red; }</style></svg>`,
		},
		{
			name: "svg expressions",
			source: `---
const title = 'icon';
---
<svg>{title ?? null}</svg>`,
		},
		{
			name: "advanced svg expression",
			source: `---
const title = 'icon';
---
<svg>{title ? <title>{title}</title> : null}</svg>`,
		},
		{
			name:   "Empty script",
			source: `<script hoist></script>`,
		},
		{
			name:   "Empty style",
			source: `<style define:vars={{ color: "Gainsboro" }}></style>`,
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
		},
		{
			name: "Mixed style siblings",
			source: `<head>
	<style is:global>div { color: red }</style>
	<style is:scoped>div { color: green }</style>
	<style>div { color: blue }</style>
</head>
<div />`,
		},
		{
			name:   "spread with double quotation marks",
			source: `<div {...propsFn("string")}/>`,
		},
		{
			name:   "class with spread",
			source: `<div class="something" {...Astro.props} />`,
		},
		{
			name:   "class:list with spread",
			source: `<div class:list="something" {...Astro.props} />`,
		},
		{
			name:   "class list",
			source: `<div class:list={['one', 'variable']} />`,
		},
		{
			name:   "class and class list simple array",
			source: `<div class="two" class:list={['one', 'variable']} />`,
		},
		{
			name:   "class and class list object",
			source: `<div class="two three" class:list={['hello goodbye', { hello: true, world: true }]} />`,
		},
		{
			name:   "class and class list set",
			source: `<div class="two three" class:list={[ new Set([{hello: true, world: true}]) ]} />`,
		},
		{
			name:   "spread without style or class",
			source: `<div {...Astro.props} />`,
		},
		{
			name:   "spread with style but no explicit class",
			source: `<style>div { color: red; }</style><div {...Astro.props} />`,
		},
		{
			name:   "Fragment",
			source: `<body><Fragment><div>Default</div><div>Named</div></Fragment></body>`,
		},
		{
			name:   "Fragment shorthand",
			source: `<body><><div>Default</div><div>Named</div></></body>`,
		},
		{
			name:   "Fragment shorthand only",
			source: `<>Hello</>`,
		},
		{
			name:   "Fragment literal only",
			source: `<Fragment>world</Fragment>`,
		},
		{
			name:   "Fragment slotted",
			source: `<body><Component><><div>Default</div><div>Named</div></></Component></body>`,
		},
		{
			name:   "Fragment slotted with name",
			source: `<body><Component><Fragment slot=named><div>Default</div><div>Named</div></Fragment></Component></body>`,
		},
		{
			name:   "Preserve slots inside custom-element",
			source: `<body><my-element><div slot=name>Name</div><div>Default</div></my-element></body>`,
		},
		{
			name:   "Preserve namespaces",
			source: `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><rect xlink:href="#id"></svg>`,
		},
		{
			name:   "Preserve namespaces in expressions",
			source: `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><rect xlink:href={` + BACKTICK + `#${iconId}` + BACKTICK + `}></svg>`,
		},
		{
			name:   "Preserve namespaces for components",
			source: `<Component some:thing="foobar">`,
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
		},
		{
			name: "import.meta",
			source: `---
const components = import.meta.glob("../components/*.astro", {
  import: 'default'
});
---`,
		},
		{
			name:   "doctype",
			source: `<!DOCTYPE html><div/>`,
		},
		{
			name: "select option expression",
			source: `---
const value = 'test';
---
<select><option>{value}</option></select>`,
		},
		{
			name: "select nested option",
			source: `---
const value = 'test';
---
<select>{value && <option>{value}</option>}</select>`,
		},
		{
			name:   "select map expression",
			source: `<select>{[1, 2, 3].map(num => <option>{num}</option>)}</select><div>Hello world!</div>`,
		},
		{
			name: "textarea",
			source: `---
const value = 'test';
---
<textarea>{value}</textarea>`,
		},
		{
			name:   "textarea inside expression",
			source: `{bool && <textarea>{value}</textarea>} {!bool && <input>}`,
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
		},
		{
			name: "table expressions (no implicit tbody)",
			source: `---
const items = ["Dog", "Cat", "Platipus"];
---
<table>{items.map(item => (<tr><td>{item}</td></tr>))}</table>`,
		},
		{
			name:   "table caption expression",
			source: `<table><caption>{title}</caption><tr><td>Hello</td></tr></table>`,
		},
		{
			name:   "table expression with trailing div",
			source: `<table><tr><td>{title}</td></tr></table><div>Div</div>`,
		},
		{
			name: "tbody expressions",
			source: `---
const items = ["Dog", "Cat", "Platipus"];
---
<table><tr><td>Name</td></tr>{items.map(item => (<tr><td>{item}</td></tr>))}</table>`,
		},
		{
			name: "tbody expressions 2",
			source: `---
const items = ["Dog", "Cat", "Platipus"];
---
<table><tr><td>Name</td></tr>{items.map(item => (<tr><td>{item}</td><td>{item + 's'}</td></tr>))}</table>`,
		},
		{
			name:   "tbody expressions 3",
			source: `<table><tbody>{rows.map(row => (<tr><td><strong>{row}</strong></td></tr>))}</tbody></table>`,
		},
		{
			name:   "td expressions",
			source: `<table><tr><td><h2>Row 1</h2></td><td>{title}</td></tr></table>`,
		},
		{
			name:   "td expressions II",
			source: `<table>{data.map(row => <tr>{row.map(cell => <td>{cell}</td>)}</tr>)}</table>`,
		},
		{
			name:   "self-closing td",
			source: `<table>{data.map(row => <tr>{row.map(cell => <td set:html={cell} />)}</tr>)}</table>`,
		},
		{
			name:   "th expressions",
			source: `<table><thead><tr><th>{title}</th></tr></thead></table>`,
		},
		{
			name:   "tr only",
			source: `<tr><td>col 1</td><td>col 2</td><td>{foo}</td></tr>`,
		},
		{
			name:   "caption only",
			source: `<caption>Hello world!</caption>`,
		},
		{
			name:   "anchor expressions",
			source: `<a>{expr}</a>`,
		},
		{
			name:   "anchor inside expression",
			source: `{true && <a>expr</a>}`,
		},
		{
			name:   "anchor content",
			source: `<a><div><h3></h3><ul><li>{expr}</li></ul></div></a>`,
		},
		{
			name:   "small expression",
			source: `<div><small>{a}</small>{data.map(a => <Component value={a} />)}</div>`,
		},
		{
			name:   "division inside expression",
			source: `<div>{16 / 4}</div>`,
		},
		{
			name:   "escaped entity",
			source: `<img alt="A person saying &#x22;hello&#x22;">`,
		},
		{
			name:   "textarea in form",
			source: `<html><Component><form><textarea></textarea></form></Component></html>`,
		},
		{
			name:   "select in form",
			source: `<form><select>{options.map((option) => (<option value={option.id}>{option.title}</option>))}</select><div><label>Title 3</label><input type="text" /></div><button type="submit">Submit</button></form>`,
		},
		{
			name:   "Expression in form followed by other sibling forms",
			source: "<form><p>No expression here. So the next form will render.</p></form><form><h3>{data.formLabelA}</h3></form><form><h3>{data.formLabelB}</h3></form><form><p>No expression here, but the last form before me had an expression, so my form didn't render.</p></form><form><h3>{data.formLabelC}</h3></form><div><p>Here is some in-between content</p></div><form><h3>{data.formLabelD}</h3></form>",
		},
		{
			name:   "slot inside of Base",
			source: `<Base title="Home"><div>Hello</div></Base>`,
		},
		{
			name:   "user-defined `implicit` is printed",
			source: `<html implicit></html>`,
		},
		{
			name: "css comment doesn’t produce semicolon",
			source: `<style>/* comment */.container {
    padding: 2rem;
	}
</style><div class="container">My Text</div>`,
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
		},
		{
			name:   "table",
			source: "<table><tr>{[0,1,2].map(x => (<td>{x}</td>))}</tr></table>",
		},
		{
			name:   "table II",
			source: "<table><thead><tr>{['Hey','Ho'].map((item)=> <th scope=\"col\">{item}</th>)}</tr></thead></table>",
		},
		{
			name:   "table III",
			source: "<table><tbody><tr><td>Cell</td><Cell /><Cell /><Cell /></tr></tbody></table>",
		},
		{
			name:   "table IV",
			source: "<body><div><tr><td>hello world</td></tr></div></body>",
		},
		{
			name:   "table slot I",
			source: "<table><slot /></table>",
		},
		{
			name:   "table slot II",
			source: "<table><tr><slot /></tr></table>",
		},
		{
			name:   "table slot III",
			source: "<table><td><slot /></td></table>",
		},
		{
			name:   "table slot IV",
			source: "<table><thead><slot /></thead></table>",
		},
		{
			name:   "table slot V",
			source: "<table><tbody><slot /></tbody></table>",
		},
		{
			name:   "XElement",
			source: `<XElement {...attrs}></XElement>{onLoadString ? <script data-something></script> : null }`,
		},
		{
			name:   "Empty expression",
			source: "<body>({})</body>",
		},
		{
			name:   "Empty expression with whitespace",
			source: "<body>({   })</body>",
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
		},
		{
			name:   "Empty attribute expression",
			source: "<body attr={}></body>",
		},
		{
			name:   "is:raw",
			source: "<article is:raw><% awesome %></article>",
		},
		{
			name:   "Component is:raw",
			source: "<Component is:raw>{<% awesome %>}</Component>",
		},
		{
			name:   "set:html",
			source: "<article set:html={content} />",
		},
		{
			name:   "set:html with quoted attribute",
			source: `<article set:html="content" />`,
		},
		{
			name:   "set:html with template literal attribute without variable",
			source: `<article set:html=` + BACKTICK + `content` + BACKTICK + ` />`,
		},
		{
			name:   "set:html with template literal attribute with variable",
			source: `<article set:html=` + BACKTICK + `${content}` + BACKTICK + ` />`,
		},
		{
			name:   "set:text",
			source: "<article set:text={content} />",
		},
		{
			name:   "set:text with quoted attribute",
			source: `<article set:text="content" />`,
		},
		{
			name:   "set:text with template literal attribute without variable",
			source: `<article set:text=` + BACKTICK + `content` + BACKTICK + ` />`,
		},
		{
			name:   "set:text with template literal attribute with variable",
			source: `<article set:text=` + BACKTICK + `${content}` + BACKTICK + ` />`,
		},
		{
			name:   "set:html on Component",
			source: `<Component set:html={content} />`,
		},
		{
			name:   "set:html on Component with quoted attribute",
			source: `<Component set:html="content" />`,
		},
		{
			name:   "set:html on Component with template literal attribute without variable",
			source: `<Component set:html=` + BACKTICK + `content` + BACKTICK + ` />`,
		},
		{
			name:   "set:html on Component with template literal attribute with variable",
			source: `<Component set:html=` + BACKTICK + `${content}` + BACKTICK + ` />`,
		},
		{
			name:   "set:text on Component",
			source: "<Component set:text={content} />",
		},
		{
			name:   "set:text on Component with quoted attribute",
			source: `<Component set:text="content" />`,
		},
		{
			name:   "set:text on Component with template literal attribute without variable",
			source: `<Component set:text=` + BACKTICK + `content` + BACKTICK + ` />`,
		},
		{
			name:   "set:text on Component with template literal attribute with variable",
			source: `<Component set:text=` + BACKTICK + `${content}` + BACKTICK + ` />`,
		},
		{
			name:   "set:html on custom-element",
			source: "<custom-element set:html={content} />",
		},
		{
			name:   "set:html on custom-element with quoted attribute",
			source: `<custom-element set:html="content" />`,
		},
		{
			name:   "set:html on custom-element with template literal attribute without variable",
			source: `<custom-element set:html=` + BACKTICK + `content` + BACKTICK + ` />`,
		},
		{
			name:   "set:html on custom-element with template literal attribute with variable",
			source: `<custom-element set:html=` + BACKTICK + `${content}` + BACKTICK + ` />`,
		},
		{
			name:   "set:text on custom-element",
			source: "<custom-element set:text={content} />",
		},
		{
			name:   "set:text on custom-element with quoted attribute",
			source: `<custom-element set:text="content" />`,
		},
		{
			name:   "set:text on custom-element with template literal attribute without variable",
			source: `<custom-element set:text=` + BACKTICK + `content` + BACKTICK + ` />`,
		},
		{
			name:   "set:text on custom-element with template literal attribute with variable",
			source: `<custom-element set:text=` + BACKTICK + `${content}` + BACKTICK + ` />`,
		},
		{
			name:   "set:html on self-closing tag",
			source: "<article set:html={content} />",
		},
		{
			name:   "set:html on self-closing tag with quoted attribute",
			source: `<article set:html="content" />`,
		},
		{
			name:   "set:html on self-closing tag with template literal attribute without variable",
			source: `<article set:html=` + BACKTICK + `content` + BACKTICK + ` />`,
		},
		{
			name:   "set:html on self-closing tag with template literal attribute with variable",
			source: `<article set:html=` + BACKTICK + `${content}` + BACKTICK + ` />`,
		},
		{
			name:   "set:html with other attributes",
			source: "<article set:html={content} cool=\"true\" />",
		},
		{
			name:   "set:html with quoted attribute and other attributes",
			source: `<article set:html="content" cool="true" />`,
		},
		{
			name:   "set:html with template literal attribute without variable and other attributes",
			source: `<article set:html=` + BACKTICK + `content` + BACKTICK + ` cool="true" />`,
		},
		{
			name:   "set:html with template literal attribute with variable and other attributes",
			source: `<article set:html=` + BACKTICK + `${content}` + BACKTICK + ` cool="true" />`,
		},
		{
			name:   "set:html on empty tag",
			source: "<article set:html={content}></article>",
		},
		{
			name:   "set:html on empty tag with quoted attribute",
			source: `<article set:html="content"></article>`,
		},
		{
			name:   "set:html on empty tag with template literal attribute without variable",
			source: `<article set:html=` + BACKTICK + `content` + BACKTICK + `></article>`,
		},
		{
			name:   "set:html on empty tag with template literal attribute with variable",
			source: `<article set:html=` + BACKTICK + `${content}` + BACKTICK + `></article>`,
		},
		{
			// If both "set:*" directives are passed, we only respect the first one
			name:   "set:html and set:text",
			source: "<article set:html={content} set:text={content} />",
		},
		//
		{
			name:   "set:html on tag with children",
			source: "<article set:html={content}>!!!</article>",
		},
		{
			name:   "set:html on tag with children and quoted attribute",
			source: `<article set:html="content">!!!</article>`,
		},
		{
			name:   "set:html on tag with children and template literal attribute without variable",
			source: `<article set:html=` + BACKTICK + `content` + BACKTICK + `>!!!</article>`,
		},
		{
			name:   "set:html on tag with children and template literal attribute with variable",
			source: `<article set:html=` + BACKTICK + `${content}` + BACKTICK + `>!!!</article>`,
		},
		{
			name:   "set:html on tag with empty whitespace",
			source: "<article set:html={content}>   </article>",
		},
		{
			name:   "set:html on tag with empty whitespace and quoted attribute",
			source: `<article set:html="content">   </article>`,
		},
		{
			name:   "set:html on tag with empty whitespace and template literal attribute without variable",
			source: `<article set:html=` + BACKTICK + `content` + BACKTICK + `>   </article>`,
		},
		{
			name:   "set:html on tag with empty whitespace and template literal attribute with variable",
			source: `<article set:html=` + BACKTICK + `${content}` + BACKTICK + `>   </article>`,
		},
		{
			name:   "set:html on script",
			source: "<script set:html={content} />",
		},
		{
			name:   "set:html on script with quoted attribute",
			source: `<script set:html="alert(1)" />`,
		},
		{
			name:   "set:html on script with template literal attribute without variable",
			source: `<script set:html=` + BACKTICK + `alert(1)` + BACKTICK + ` />`,
		},
		{
			name:   "set:html on script with template literal attribute with variable",
			source: `<script set:html=` + BACKTICK + `${content}` + BACKTICK + ` />`,
		},
		{
			name:   "set:html on style",
			source: "<style set:html={content} />",
		},
		{
			name:   "set:html on style with quoted attribute",
			source: `<style set:html="h1{color:green;}" />`,
		},
		{
			name:   "set:html on style with template literal attribute without variable",
			source: `<style set:html=` + BACKTICK + `h1{color:green;}` + BACKTICK + ` />`,
		},
		{
			name:   "set:html on style with template literal attribute with variable",
			source: `<style set:html=` + BACKTICK + `${content}` + BACKTICK + ` />`,
		},
		{
			name:   "set:html on Fragment",
			source: "<Fragment set:html={\"<p>&#x3C;i>This should NOT be italic&#x3C;/i></p>\"} />",
		},
		{
			name:   "set:html on Fragment with quoted attribute",
			source: "<Fragment set:html=\"<p>&#x3C;i>This should NOT be italic&#x3C;/i></p>\" />",
		},
		{
			name:   "set:html on Fragment with template literal attribute without variable",
			source: "<Fragment set:html=`<p>&#x3C;i>This should NOT be italic&#x3C;/i></p>` />",
		},
		{
			name:   "set:html on Fragment with template literal attribute with variable",
			source: `<Fragment set:html=` + BACKTICK + `${content}` + BACKTICK + ` />`,
		},
		{
			name:   "template literal attribute on component",
			source: `<Component class=` + BACKTICK + `red` + BACKTICK + ` />`,
		},
		{
			name:   "template literal attribute with variable on component",
			source: `<Component class=` + BACKTICK + `${color}` + BACKTICK + ` />`,
		},
		{
			name:   "define:vars on style",
			source: "<style>h1{color:green;}</style><style define:vars={{color:'green'}}>h1{color:var(--color)}</style><h1>testing</h1>",
		},
		{
			name:   "define:vars on style tag with style shorthand attribute on element",
			source: "<style define:vars={{color:'green'}}>h1{color:var(--color)}</style><h1 {style}>testing</h1>",
		},
		{
			name:   "define:vars on style tag with style expression attribute on element",
			source: "<style define:vars={{color:'green'}}>h1{color:var(--color)}</style><h1 style={myStyles}>testing</h1>",
		},
		{
			name:   "define:vars on style tag with style empty attribute on element",
			source: "<style define:vars={{color:'green'}}>h1{color:var(--color)}</style><h1 style>testing</h1>",
		},
		{
			name:   "define:vars on style tag with style quoted attribute on element",
			source: "<style define:vars={{color:'green'}}>h1{color:var(--color)}</style><h1 style='color: yellow;'>testing</h1>",
		},
		{
			name:   "define:vars on style tag with style template literal attribute on element",
			source: "<style define:vars={{color:'green'}}>h1{color:var(--color)}</style><h1 style=`color: ${color};`>testing</h1>",
		},
		{
			name:   "multiple define:vars on style",
			source: "<style define:vars={{color:'green'}}>h1{color:var(--color)}</style><style define:vars={{color:'red'}}>h2{color:var(--color)}</style><h1>foo</h1><h2>bar</h2>",
		},
		{
			name:   "define:vars on non-root elements",
			source: "<style define:vars={{color:'green'}}>h1{color:var(--color)}</style>{true ? <h1>foo</h1> : <h1>bar</h1>}",
		},
		{
			name: "define:vars on script with StaticExpression turned on",
			// 1. An inline script with is:inline - right
			// 2. A hoisted script - wrong, shown up in scripts.add
			// 3. A define:vars hoisted script
			// 4. A define:vars inline script
			source: `<script is:inline>var one = 'one';</script><script>var two = 'two';</script><script define:vars={{foo:'bar'}}>var three = foo;</script><script is:inline define:vars={{foo:'bar'}}>var four = foo;</script>`,
		},
		{
			name: "define:vars on a module script with imports",
			// Should not wrap with { } scope.
			source: `<script type="module" define:vars={{foo:'bar'}}>import 'foo';\nvar three = foo;</script>`,
		},
		{
			name:   "comments removed from attribute list",
			source: `<div><h1 {/* comment 1 */} value="1" {/* comment 2 */}>Hello</h1><Component {/* comment 1 */} value="1" {/* comment 2 */} /></div>`,
		},
		{
			name:   "includes comments for shorthand attribute",
			source: `<div><h1 {/* comment 1 */ id /* comment 2 */}>Hello</h1><Component {/* comment 1 */ id /* comment 2 */}/></div>`,
		},
		{
			name:   "includes comments for expression attribute",
			source: `<div><h1 attr={/* comment 1 */ isTrue ? 1 : 2 /* comment 2 */}>Hello</h1><Component attr={/* comment 1 */ isTrue ? 1 : 2 /* comment 2 */}/></div>`,
		},
		{
			name:   "comment only expressions are removed I",
			source: `{/* a comment 1 */}<h1>{/* a comment 2*/}Hello</h1>`,
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
		},
		{
			name:   "component with only a script",
			source: "<script>console.log('hello world');</script>",
		},
		{
			name:     "passes filename into createComponent if passed into the compiler options",
			source:   `<div>test</div>`,
			filename: "/projects/app/src/pages/page.astro",
		},
		{
			name:     "passes escaped filename into createComponent if it contains single quotes",
			source:   `<div>test</div>`,
			filename: "/projects/app/src/pages/page-with-'-quotes.astro",
		},
		{
			name:     "maybeRenderHead not printed for hoisted scripts",
			source:   `<script></script><Layout></Layout>`,
			filename: "/projects/app/src/pages/page.astro",
		},
		{
			name:     "complex recursive component",
			source:   `{(<Fragment><Fragment set:html={` + BACKTICK + `<${Node.tag} ${stringifyAttributes(Node.attributes)}>` + BACKTICK + `} />{Node.children.map((child) => (<Astro.self node={child} />))}<Fragment set:html={` + BACKTICK + `</${Node.tag}>` + BACKTICK + `} /></Fragment>)}`,
			filename: "/projects/app/src/components/RenderNode.astro",
		},
		{
			name:   "multibyte character + style",
			source: `<style>a { font-size: 16px; }</style><a class="test">ツ</a>`,
		},
		{
			name: "multibyte characters",
			source: `---
---
<h1>こんにちは</h1>`,
		},

		{
			name:   "multibyte character + script",
			source: `<script>console.log('foo')</script><a class="test">ツ</a>`,
		},

		{
			name:        "transition:name with an expression",
			source:      `<div transition:name={one + '-' + 'two'}></div>`,
			filename:    "/projects/app/src/pages/page.astro",
			transitions: true,
		},
		{
			name:        "transition:name with an template literal",
			source:      "<div transition:name=`${one}-two`></div>",
			filename:    "/projects/app/src/pages/page.astro",
			transitions: true,
		},
		{
			name:        "transition:animate with an expression",
			source:      "<div transition:animate={slide({duration:15})}></div>",
			filename:    "/projects/app/src/pages/page.astro",
			transitions: true,
		},
		{
			name:        "transition:animate on Component",
			source:      `<Component class="bar" transition:animate="morph"></Component>`,
			filename:    "/projects/app/src/pages/page.astro",
			transitions: true,
		},
		{
			name:        "transition:persist converted to a data attribute",
			source:      `<div transition:persist></div>`,
			transitions: true,
		},
		{
			name:        "transition:persist uses transition:name if defined",
			source:      `<div transition:persist transition:name="foo"></div>`,
			transitions: true,
		},
		{
			name:        "transition:persist-props converted to a data attribute",
			source:      `<my-island transition:persist transition:persist-props="false"></my-island>`,
			transitions: true,
		},
		{
			name:   "trailing expression",
			source: `<Component />{}`,
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

			test_utils.MakeSnapshot(
				&test_utils.SnapshotOptions{
					Testing:      t,
					TestCaseName: tt.name,
					Input:        code,
					Output:       output,
					Kind:         test_utils.JsOutput,
					FolderName:   "__printer_js__",
				})
		})
	}
}

func TestPrintToJSON(t *testing.T) {
	tests := []jsonTestcase{
		{
			name:   "basic",
			source: `<h1>Hello world!</h1>`,
		},
		{
			name:   "expression",
			source: `<h1>Hello {world}</h1>`,
		},
		{
			name:   "Component",
			source: `<Component />`,
		},
		{
			name:   "custom-element",
			source: `<custom-element />`,
		},
		{
			name:   "Doctype",
			source: `<!DOCTYPE html />`,
		},
		{
			name:   "Comment",
			source: `<!--hello-->`,
		},
		{
			name:   "Comment preserves whitespace",
			source: `<!-- hello -->`,
		},
		{
			name:   "Fragment Shorthand",
			source: `<>Hello</>`,
		},
		{
			name:   "Fragment Literal",
			source: `<Fragment>World</Fragment>`,
		},
		{
			name: "Frontmatter",
			source: `---
const a = "hey"
---
<div>{a}</div>`,
		},
		{
			name: "JSON escape",
			source: `---
const a = "\n"
const b = "\""
const c = '\''
---
{a + b + c}`,
		},
		{
			name:   "Preserve namespaces",
			source: `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><rect xlink:href="#id"></svg>`,
		},
		{
			name:   "style before html",
			source: `<style></style><html><body><h1>Hello world!</h1></body></html>`,
		},
		{
			name:   "style after html",
			source: `<html><body><h1>Hello world!</h1></body></html><style></style>`,
		},
		{
			name:   "style after empty html",
			source: `<html></html><style></style>`,
		},
		{
			name:   "style after html with component in head",
			source: `<html lang="en"><head><BaseHead /></head></html><style>@use "../styles/global.scss";</style>`,
		},
		{
			name:   "style after html with component in head and body",
			source: `<html lang="en"><head><BaseHead /></head><body><Header /></body></html><style>@use "../styles/global.scss";</style>`,
		},
		{
			name:   "style after body with component in head and body",
			source: `<html lang="en"><head><BaseHead /></head><body><Header /></body><style>@use "../styles/global.scss";</style></html>`,
		},
		{
			name:   "style in html",
			source: `<html><body><h1>Hello world!</h1></body><style></style></html>`,
		},
		{
			name:   "style in body",
			source: `<html><body><h1>Hello world!</h1><style></style></body></html>`,
		},
		{
			name:   "element with unterminated double quote attribute",
			source: `<main id="gotcha />`,
		},
		{
			name:   "element with unterminated single quote attribute",
			source: `<main id='gotcha />`,
		},
		{
			name:   "element with unterminated template literal attribute",
			source: `<main id=` + BACKTICK + `gotcha />`,
		},
	}

	for _, tt := range tests {
		if tt.only {
			tests = make([]jsonTestcase, 0)
			tests = append(tests, tt)
			break
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// transform output from source
			code := test_utils.Dedent(tt.source)

			doc, err := astro.ParseWithOptions(strings.NewReader(code), astro.ParseOptionEnableLiteral(true), astro.ParseOptionWithHandler(&handler.Handler{}))
			if err != nil {
				t.Error(err)
			}

			result := PrintToJSON(code, doc, types.ParseOptions{Position: false})

			test_utils.MakeSnapshot(
				&test_utils.SnapshotOptions{
					Testing:      t,
					TestCaseName: tt.name,
					Input:        code,
					Output:       string(result.Output),
					Kind:         test_utils.JsonOutput,
					FolderName:   "__printer_json__",
				})
		})
	}
}
