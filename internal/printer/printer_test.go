package printer

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	tycho "github.com/snowpackjs/astro/internal"
	"github.com/snowpackjs/astro/internal/test_utils"
	"github.com/snowpackjs/astro/internal/transform"
)

var INTERNAL_IMPORTS = fmt.Sprintf("import {\n  %s\n} from \"%s\";\n", strings.Join([]string{
	"render as " + TEMPLATE_TAG,
	"createAstro as " + CREATE_ASTRO,
	"createComponent as " + CREATE_COMPONENT,
	"renderComponent as " + RENDER_COMPONENT,
	"renderSlot as " + RENDER_SLOT,
	"addAttribute as " + ADD_ATTRIBUTE,
	"spreadAttributes as " + SPREAD_ATTRIBUTES,
	"defineStyleVars as " + DEFINE_STYLE_VARS,
	"defineScriptVars as " + DEFINE_SCRIPT_VARS,
	"createMetadata as " + CREATE_METADATA,
}, ",\n  "), "http://localhost:3000/")
var PRELUDE = fmt.Sprintf(`//@ts-ignore
const $$Component = %s(async ($$result, $$props, %s) => {
const Astro = $$result.createAstro($$Astro, $$props, %s);%s`, CREATE_COMPONENT, SLOTS, SLOTS, "\n")
var RETURN = fmt.Sprintf("return %s%s", TEMPLATE_TAG, BACKTICK)
var SUFFIX = fmt.Sprintf("%s;", BACKTICK) + `
});
export default $$Component;`
var STYLE_PRELUDE = "const STYLES = [\n"
var STYLE_SUFFIX = "];\n$$result.styles.add(...STYLES);\n"
var SCRIPT_PRELUDE = "const SCRIPTS = [\n"
var SCRIPT_SUFFIX = "];\n$$result.scripts.add(...SCRIPTS);\n"
var CREATE_ASTRO_CALL = "const $$Astro = $$createAstro(import.meta.url, 'https://astro.build');\nconst Astro = $$Astro;"

// SPECIAL TEST FIXTURES
var NON_WHITESPACE_CHARS = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[];:'\",.?")

type want struct {
	imports     string
	frontmatter []string
	styles      []string
	metadata    string
	code        string
	scripts     []string
}

type testcase struct {
	name   string
	source string
	only   bool
	want   want
}

func TestPrinter(t *testing.T) {
	longRandomString := ""
	for i := 0; i < 4080; i++ {
		longRandomString += string(NON_WHITESPACE_CHARS[rand.Intn(len(NON_WHITESPACE_CHARS))])
	}

	tests := []testcase{
		{
			name:   "basic (no frontmatter)",
			source: `<button>Click</button>`,
			want: want{
				imports:     "",
				frontmatter: []string{},
				styles:      []string{},
				code:        `<html><head></head><body><button>Click</button></body></html>`,
			},
		},
		{
			name: "basic (frontmatter)",
			source: `---
const href = '/about';
---
<a href={href}>About</a>`,
			want: want{
				imports:     "",
				frontmatter: []string{"", "const href = '/about';"},
				styles:      []string{},
				code:        `<html><head></head><body><a${` + ADD_ATTRIBUTE + `(href, "href")}>About</a></body></html>`,
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
				imports: "",
				frontmatter: []string{
					`import VueComponent from '../components/Vue.vue';

import * as $$module1 from '../components/Vue.vue';`,
				},
				styles:   []string{},
				metadata: `{ modules: [{ module: $$module1, specifier: '../components/Vue.vue' }], hydratedComponents: [], hoisted: [] }`,
				code: `<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    ${` + RENDER_COMPONENT + `($$result,'VueComponent',VueComponent,{})}
  </body></html>`,
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
				imports: "",
				frontmatter: []string{
					`import * as ns from '../components';

import * as $$module1 from '../components';`,
				},
				styles:   []string{},
				metadata: `{ modules: [{ module: $$module1, specifier: '../components' }], hydratedComponents: [], hoisted: [] }`,
				code: `<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    ${` + RENDER_COMPONENT + `($$result,'ns.Component',ns.Component,{})}
  </body></html>`,
			},
		},
		{
			name:   "conditional render",
			source: `<body>{false ? <div>#f</div> : <div>#t</div>}</body>`,
			want: want{
				imports:     "",
				frontmatter: []string{},
				styles:      []string{},
				code:        "<html><head></head><body>${false ? $$render`<div>#f</div>` : $$render`<div>#t</div>`}</body></html>",
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
				imports:     "",
				frontmatter: []string{"", "const items = [0, 1, 2];"},
				styles:      []string{},
				code: fmt.Sprintf(`<html><head></head><body><ul>
	${items.map(item => {
		return $$render%s<li>${item}</li>%s;
	})}
</ul></body></html>`, BACKTICK, BACKTICK),
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
				imports:     "",
				frontmatter: []string{"", "const groups = [[0, 1, 2], [3, 4, 5]];"},
				styles:      []string{},
				code: fmt.Sprintf(`<html><head></head><body><div>
	${groups.map(items => {
		return %s<ul>${
			items.map(item => {
				return %s<li>${item}</li>%s;
			})
		}</ul>%s})}
</div></body></html>`, "$$render"+BACKTICK, "$$render"+BACKTICK, BACKTICK, BACKTICK),
			},
		},
		{
			name:   "backtick in HTML comment",
			source: "<body><!-- `npm install astro` --></body>",
			want: want{
				imports:     "",
				frontmatter: []string{},
				styles:      []string{},
				code:        "<html><head></head><body><!-- \\`npm install astro\\` --></body></html>",
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
				imports:     "",
				frontmatter: []string{"", "const items = ['red', 'yellow', 'blue'];"},
				styles:      []string{},
				code: `<html><head></head><body><div>
  ${items.map((item) => (
    // foo < > < }
$$render` + "`" + `<div${$$addAttribute(color, "id")}>color</div>` + "`" + `
  ))}
  ${items.map((item) => (
    /* foo < > < } */$$render` + "`" + `<div${$$addAttribute(color, "id")}>color</div>` + "`" + `
  ))}
</div></body></html>`,
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
				imports: "",
				frontmatter: []string{`import Component from "test";

import * as $$module1 from 'test';`},
				styles:   []string{},
				metadata: `{ modules: [{ module: $$module1, specifier: 'test' }], hydratedComponents: [], hoisted: [] }`,
				code:     `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + "`" + `<div>Default</div>` + "`" + `,"named": () => $$render` + "`" + `<div>Named</div>` + "`" + `,})}`,
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
				imports: "",
				frontmatter: []string{`import Component from 'test';

import * as $$module1 from 'test';`},
				styles:   []string{},
				metadata: `{ modules: [{ module: $$module1, specifier: 'test' }], hydratedComponents: [], hoisted: [] }`,
				code:     `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + "`" + `<div>Default</div>` + "`" + `,"named": () => $$render` + "`" + `<div>Named</div>` + "`" + `,})}`,
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
				imports:     "",
				frontmatter: []string{``, `const name = "world";`},
				styles:      []string{},
				code: `<html>
  <head>
    <title>Hello ${name}</title>
  </head>
  <body>
    <div></div>
  </body></html>`,
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
				imports:     "",
				frontmatter: []string{},
				styles:      []string{"{props:{\"data-astro-id\":\"DPOHFLYM\"},children:`.title.astro-DPOHFLYM{font-family:fantasy;font-size:28px;}.body.astro-DPOHFLYM{font-size:1em;}`}"},
				code: `<html class="astro-DPOHFLYM"><head>

		</head><body><h1 class="title astro-DPOHFLYM">Page Title</h1>
		<p class="body astro-DPOHFLYM">I’m a page</p></body></html>`,
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
  <script src="js/scripts.js"></script>
  </body>
</html>`,
			want: want{
				imports:     "",
				frontmatter: []string{},
				styles:      []string{},
				code: `<!DOCTYPE html><html lang="en">
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
  <script src="js/scripts.js"></script>
  </body></html>`,
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
				imports: "",
				frontmatter: []string{`// Component Imports
import Counter from '../components/Counter.jsx'

import * as $$module1 from '../components/Counter.jsx';`,
					`const someProps = {
  count: 0,
}

// Full Astro Component Syntax:
// https://docs.astro.build/core-concepts/astro-components/`},
				styles:   []string{fmt.Sprintf(`{props:{"data-astro-id":"HMNNHVCQ"},children:%s:root{font-family:system-ui;padding:2em 0;}.counter{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));place-items:center;font-size:2em;margin-top:2em;}.children{display:grid;place-items:center;margin-bottom:2em;}%s}`, BACKTICK, BACKTICK)},
				metadata: `{ modules: [{ module: $$module1, specifier: '../components/Counter.jsx' }], hydratedComponents: [Counter], hoisted: [] }`,
				code: `<html lang="en" class="astro-HMNNHVCQ">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width">
    <link rel="icon" type="image/x-icon" href="/favicon.ico">

  </head>
  <body>
    <main class="astro-HMNNHVCQ">
      ${$$renderComponent($$result,'Counter',Counter,{...(someProps),"client:visible":true,"client:component-path":($$metadata.getPath(Counter)),"client:component-export":($$metadata.getExport(Counter)),"class":"astro-HMNNHVCQ"},{"default": () => $$render` + "`" + `<h1 class="astro-HMNNHVCQ">Hello React!</h1>` + "`" + `,})}
    </main>
  </body></html>`,
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
				imports: "",
				frontmatter: []string{`import Widget from '../components/Widget.astro';
import Widget2 from '../components/Widget2.astro';

import * as $$module1 from '../components/Widget.astro';
import * as $$module2 from '../components/Widget2.astro';`},
				styles:   []string{},
				metadata: `{ modules: [{ module: $$module1, specifier: '../components/Widget.astro' }, { module: $$module2, specifier: '../components/Widget2.astro' }], hydratedComponents: [], hoisted: [] }`,
				code: `<html lang="en">
  <head>
    <script type="module" src="/regular_script.js"></script>
  </head><body></body></html>`,
			},
		},
		{
			name: "script hoist with frontmatter",
			source: `---
---
<script type="module" hoist>console.log("Hello");</script>`,
			want: want{
				imports:     "",
				frontmatter: []string{""},
				styles:      []string{},
				metadata:    `{ modules: [], hydratedComponents: [], hoisted: [{ type: 'inline', value: 'console.log("Hello");' }] }`,
				scripts:     []string{fmt.Sprintf(`{props:{"type":"module","hoist":true},children:%sconsole.log("Hello");%s}`, BACKTICK, BACKTICK)},
				code:        `<html><head></head><body></body></html>`,
			},
		},
		{
			name: "script hoist remote",
			source: `---
---
<script type="module" hoist src="url" />`,
			want: want{
				imports:     "",
				frontmatter: []string{"\n"},
				styles:      []string{},
				metadata:    `{ modules: [], hydratedComponents: [], hoisted: [{ type: 'remote', src: 'url' }] }`,
				scripts:     []string{`{props:{"type":"module","hoist":true,"src":"url"}}`},
				code:        "<html><head></head><body></body></html>",
			},
		},
		{
			name: "script hoist without frontmatter",
			source: `
							<main>
								<script type="module" hoist>console.log("Hello");</script>
							`,
			want: want{
				imports:  "",
				styles:   []string{},
				scripts:  []string{"{props:{\"type\":\"module\",\"hoist\":true},children:`console.log(\"Hello\");`}"},
				metadata: `{ modules: [], hydratedComponents: [], hoisted: [{ type: 'inline', value: 'console.log("Hello");' }] }`,
				code: `<html><head></head><body><main>

</main></body></html>`,
			},
		},
		{
			name:   "script nohoist",
			source: `<main><script type="module">console.log("Hello");</script>`,
			want: want{
				code: `<html><head></head><body><main><script type="module">console.log("Hello");</script></main></body></html>`,
			},
		},
		{
			name:   "script define:vars",
			source: `<main><script define:vars={{ value: 0 }} type="module">console.log(value);</script>`,
			want: want{
				code: fmt.Sprintf(`<html><head></head><body><main><script type="module">${%s({ value: 0 })}console.log(value);</script></main></body></html>`, DEFINE_SCRIPT_VARS),
			},
		},
		{
			name:   "text after title expression",
			source: `<title>a {expr} b</title>`,
			want: want{
				imports:     "",
				frontmatter: []string{},
				styles:      []string{},
				code:        `<html><head><title>a ${expr} b</title></head><body></body></html>`,
			},
		},
		{
			name:   "text after title expressions",
			source: `<title>a {expr} b {expr} c</title>`,
			want: want{
				imports:     "",
				frontmatter: []string{},
				styles:      []string{},
				code:        `<html><head><title>a ${expr} b ${expr} c</title></head><body></body></html>`,
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
				imports: "",
				frontmatter: []string{`import Component from 'test';

import * as $$module1 from 'test';
`, `const name = 'named';`},
				styles:   []string{},
				metadata: `{ modules: [{ module: $$module1, specifier: 'test' }], hydratedComponents: [], hoisted: [] }`,
				code:     `${$$renderComponent($$result,'Component',Component,{},{[name]: () => $$render` + "`" + `<div>Named</div>` + "`" + `,})}`,
			},
		},
		{
			name:   "condition expressions at the top-level",
			source: `{cond && <span></span>}{cond && <strong></strong>}`,
			want: want{
				imports:     "",
				frontmatter: []string{},
				styles:      []string{},
				code:        "<html><head></head><body>${cond && $$render`<span></span>`}${cond && $$render`<strong></strong>`}</body></html>",
			},
		},
		{
			name:   "condition expressions at the top-level with head content",
			source: `{cond && <meta charset=utf8>}{cond && <title>My title</title>}`,
			want: want{
				imports:     "",
				frontmatter: []string{},
				styles:      []string{},
				code:        "<html><head>${cond && $$render`<meta charset=\"utf8\">`}${cond && $$render`<title>My title</title>`}</head><body></body></html>",
			},
		},
		{
			name: "custom elements",
			source: `---
import 'test';
---
<my-element></my-element>`,
			want: want{
				imports: "",
				frontmatter: []string{`import 'test';

import * as $$module1 from 'test';`},
				styles:   []string{},
				metadata: `{ modules: [{ module: $$module1, specifier: 'test' }], hydratedComponents: [], hoisted: [] }`,
				code:     `<html><head></head><body>${$$renderComponent($$result,'my-element','my-element',{})}</body></html>`,
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
				imports: "",
				frontmatter: []string{`import One from 'one';
import Two from 'two';
import 'custom-element';

import * as $$module1 from 'one';
import * as $$module2 from 'two';
import * as $$module3 from 'custom-element';`,
					`const name = 'world';`},
				styles:   []string{},
				metadata: `{ modules: [{ module: $$module1, specifier: 'one' }, { module: $$module2, specifier: 'two' }, { module: $$module3, specifier: 'custom-element' }], hydratedComponents: [One, Two, 'my-element'], hoisted: [] }`,
				code: `${$$renderComponent($$result,'One',One,{"client:load":true,"client:component-path":($$metadata.getPath(One)),"client:component-export":($$metadata.getExport(One))})}
${$$renderComponent($$result,'Two',Two,{"client:load":true,"client:component-path":($$metadata.getPath(Two)),"client:component-export":($$metadata.getExport(Two))})}
${$$renderComponent($$result,'my-element','my-element',{"client:load":true,"client:component-path":($$metadata.getPath('my-element')),"client:component-export":($$metadata.getExport('my-element'))})}`,
			},
		},
		{
			name:   "Component siblings are siblings",
			source: `<BaseHead></BaseHead><link href="test">`,
			want: want{
				imports:     "",
				frontmatter: []string{},
				styles:      []string{},
				code:        `${$$renderComponent($$result,'BaseHead',BaseHead,{})}<link href="test">`,
			},
		},
		{
			name:   "Self-closing components siblings are siblings",
			source: `<BaseHead /><link href="test">`,
			want: want{
				imports:     "",
				frontmatter: []string{},
				styles:      []string{},
				code:        `${$$renderComponent($$result,'BaseHead',BaseHead,{})}<link href="test">`,
			},
		},
		{
			name:   "Self-closing script in head works",
			source: `<html><head><script /></head><html>`,
			want: want{
				imports:     "",
				frontmatter: []string{},
				styles:      []string{},
				code:        `<html><head><script></script></head><body></body></html>`,
			},
		},
		{
			name:   "Self-closing components in head can have siblings",
			source: `<html><head><BaseHead /><link href="test"></head><html>`,
			want: want{
				imports:     "",
				frontmatter: []string{},
				styles:      []string{},
				code:        `<html><head>${$$renderComponent($$result,'BaseHead',BaseHead,{})}<link href="test"></head><body></body></html>`,
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
				imports: "",
				frontmatter: []string{"", `const image = './penguin.png';
const canonicalURL = new URL('http://example.com');`},
				styles: []string{},
				code:   "<html><head>${image && ($$render`<meta property=\"og:image\"${$$addAttribute(new URL(image, canonicalURL), \"content\")}>`)}</head><body></body></html>",
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
				imports: "",
				frontmatter: []string{"", `interface MarkdownFrontmatter {
	date: number;
	image: string;
	author: string;
}
let allPosts = Astro.fetchContent<MarkdownFrontmatter>('./post/*.md');`},
				styles: []string{},
				code:   "<html><head></head><body><div>testing</div></body></html>",
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
				imports: "",
				frontmatter: []string{
					`import AComponent from '../components/AComponent.jsx';
import ZComponent from '../components/ZComponent.jsx';

import * as $$module1 from '../components/AComponent.jsx';
import * as $$module2 from '../components/ZComponent.jsx';`},
				metadata: `{ modules: [{ module: $$module1, specifier: '../components/AComponent.jsx' }, { module: $$module2, specifier: '../components/ZComponent.jsx' }], hydratedComponents: [], hoisted: [] }`,
				styles:   []string{},
				code: `<html><head></head><body>
  ${` + RENDER_COMPONENT + `($$result,'AComponent',AComponent,{})}
  ${` + RENDER_COMPONENT + `($$result,'ZComponent',ZComponent,{})}
</body></html>`,
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
></body></html>`,
			want: want{
				imports:     ``,
				frontmatter: []string{},
				styles:      []string{},
				code:        `<html><head></head><body>` + longRandomString + `<img width="1600" height="1131" class="img" src="https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1200&q=75" srcSet="https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1200&q=75 800w,https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1200&q=75 1200w,https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1600&q=75 1600w,https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=2400&q=75 2400w" sizes="(max-width: 800px) 800px, (max-width: 1200px) 1200px, (max-width: 1600px) 1600px, (max-width: 2400px) 2400px, 1200px"></body></html>`,
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

			doc, err := tycho.Parse(strings.NewReader(code))

			if err != nil {
				t.Error(err)
			}

			hash := tycho.HashFromSource(code)
			transform.ExtractStyles(doc)
			transform.Transform(doc, transform.TransformOptions{Scope: hash}) // note: we want to test Transform in context here, but more advanced cases could be tested separately
			result := PrintToJS(code, doc, transform.TransformOptions{
				Scope:       "astro-XXXX",
				Site:        "https://astro.build",
				InternalURL: "http://localhost:3000/",
			})
			output := string(result.Output)

			toMatch := INTERNAL_IMPORTS
			if len(tt.want.frontmatter) > 0 {
				toMatch += test_utils.Dedent(tt.want.frontmatter[0])
			}
			// Default metadata
			metadata := "{ modules: [], hydratedComponents: [], hoisted: [] }"
			if len(tt.want.metadata) > 0 {
				metadata = test_utils.Dedent(tt.want.metadata)
			}
			toMatch += "\n\n" + fmt.Sprintf("export const %s = %s(import.meta.url, %s);\n\n", METADATA, CREATE_METADATA, metadata)
			toMatch += test_utils.Dedent(CREATE_ASTRO_CALL) + "\n\n"
			toMatch += test_utils.Dedent(PRELUDE) + "\n"
			if len(tt.want.frontmatter) > 1 {
				toMatch += test_utils.Dedent(tt.want.frontmatter[1])
			}
			toMatch += "\n"
			if len(tt.want.styles) > 0 {
				toMatch = toMatch + STYLE_PRELUDE
				for _, style := range tt.want.styles {
					toMatch += style + ",\n"
				}
				toMatch += STYLE_SUFFIX
			}
			if len(tt.want.scripts) > 0 {
				toMatch = toMatch + SCRIPT_PRELUDE
				for _, script := range tt.want.scripts {
					toMatch += script + ",\n"
				}
				toMatch += SCRIPT_SUFFIX
			}
			toMatch += test_utils.Dedent(fmt.Sprintf("%s%s", RETURN, tt.want.code))
			toMatch += SUFFIX

			// compare to expected string, show diff if mismatch
			if diff := ANSIDiff(test_utils.Dedent(toMatch), test_utils.Dedent(output)); diff != "" {
				t.Error(fmt.Sprintf("mismatch (-want +got):\n%s", diff))
			}
		})
	}
}

func ANSIDiff(x, y interface{}, opts ...cmp.Option) string {
	escapeCode := func(code int) string {
		return fmt.Sprintf("\x1b[%dm", code)
	}
	diff := cmp.Diff(x, y, opts...)
	if diff == "" {
		return ""
	}
	ss := strings.Split(diff, "\n")
	for i, s := range ss {
		switch {
		case strings.HasPrefix(s, "-"):
			ss[i] = escapeCode(31) + s + escapeCode(0)
		case strings.HasPrefix(s, "+"):
			ss[i] = escapeCode(32) + s + escapeCode(0)
		}
	}
	return strings.Join(ss, "\n")
}
