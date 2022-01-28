package printer

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"testing"

	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/test_utils"
	"github.com/withastro/compiler/internal/transform"
)

var INTERNAL_IMPORTS = fmt.Sprintf("import {\n  %s\n} from \"%s\";\n", strings.Join([]string{
	FRAGMENT,
	"render as " + TEMPLATE_TAG,
	"createAstro as " + CREATE_ASTRO,
	"createComponent as " + CREATE_COMPONENT,
	"renderComponent as " + RENDER_COMPONENT,
	"escapeHTML as " + ESCAPE_HTML,
	"unescapeHTML as " + UNESCAPE_HTML,
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
var STYLE_SUFFIX = "];\nfor (const STYLE of STYLES) $$result.styles.add(STYLE);\n"
var SCRIPT_PRELUDE = "const SCRIPTS = [\n"
var SCRIPT_SUFFIX = "];\nfor (const SCRIPT of SCRIPTS) $$result.scripts.add(SCRIPT);\n"
var CREATE_ASTRO_CALL = "const $$Astro = $$createAstro(import.meta.url, 'https://astro.build', '.');\nconst Astro = $$Astro;"

// SPECIAL TEST FIXTURES
var NON_WHITESPACE_CHARS = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[];:'\",.?")

type want struct {
	frontmatter    []string
	styles         []string
	scripts        []string
	getStaticPaths string
	code           string
	skipHoist      bool // HACK: sometimes `getStaticPaths()` appears in a slightly-different location. Only use this if needed!
	metadata
}

type metadata struct {
	hoisted             []string
	hydratedComponents  []string
	modules             []string
	hydrationDirectives []string
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
				code: `<html><head></head><body><button>Click</button></body></html>`,
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
				code:        `<html><head></head><body><a${` + ADD_ATTRIBUTE + `(href, "href")}>About</a></body></html>`,
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
				frontmatter: []string{`export const getStaticPaths = async () => {
	return { paths: [] }
}`, ""},
				code: `<html><head></head><body><div></div></body></html>`,
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
				code: `<html><head></head><body><div></div></body></html>`,
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
				code: `<html><head></head><body><div></div></body></html>`,
			},
		},
		{
			name: "import assertions",
			source: `---
import data from "test" assert { type: 'json' };
---
<html><head></head><body></body></html>`,
			want: want{
				frontmatter: []string{
					`import data from "test" assert { type: 'json' };`,
				},
				metadata: metadata{modules: []string{`{ module: $$module1, specifier: 'test', assert: {type:'json'} }`}},
				styles:   []string{},
				code:     `<html><head></head><body></body></html>`,
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
				frontmatter: []string{`import * as ns from '../components';`},
				styles:      []string{},
				metadata:    metadata{modules: []string{`{ module: $$module1, specifier: '../components', assert: {} }`}},
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
  <head></head>
  <body>
	<noscript>
		${` + RENDER_COMPONENT + `($$result,'Component',Component,{})}
	</noscript>
  </body></html>`,
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
					hydrationDirectives: []string{"only"},
				},
				// Specifically do NOT render any metadata here, we need to skip this import
				code: `<html>
  <head>
    <title>Hello world</title>
  </head>
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
					hydrationDirectives: []string{"only"},
				},
				// Specifically do NOT render any metadata here, we need to skip this import
				code: `<html>
  <head>
    <title>Hello world</title>
  </head>
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
					hydrationDirectives: []string{"only"},
				},
				// Specifically do NOT render any metadata here, we need to skip this import
				code: `<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    ${` + RENDER_COMPONENT + `($$result,'components.A',null,{"client:only":true,"client:component-hydration":"only","client:component-path":($$metadata.resolvePath("../components")),"client:component-export":"A"})}
  </body></html>`,
			},
		},
		{
			name:   "conditional render",
			source: `<body>{false ? <div>#f</div> : <div>#t</div>}</body>`,
			want: want{
				code: "<html><head></head><body>${false ? $$render`<div>#f</div>` : $$render`<div>#t</div>`}</body></html>",
			},
		},
		{
			name:   "simple ternary",
			source: `<body>{link ? <a href="/">{link}</a> : <div>no link</div>}</body>`,
			want: want{
				code: fmt.Sprintf(`<html><head></head><body>${link ? $$render%s<a href="/">${link}</a>%s : $$render%s<div>no link</div>%s}</body></html>`, BACKTICK, BACKTICK, BACKTICK, BACKTICK),
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
				code: fmt.Sprintf(`<html><head></head><body><ul>
	${items.map(item => {
		return $$render%s<li>${item}</li>%s;
	})}
</ul></body></html>`, BACKTICK, BACKTICK),
			},
		},
		{
			name:   "map without component",
			source: `<header><nav>{menu.map((item) => <a href={item.href}>{item.title}</a>)}</nav></header>`,
			want: want{
				code: fmt.Sprintf(`<html><head></head><body><header><nav>${menu.map((item) => $$render%s<a${$$addAttribute(item.href, "href")}>${item.title}</a>%s)}</nav></header></body></html>`, BACKTICK, BACKTICK),
			},
		},
		{
			name:   "map with component",
			source: `<header><nav>{menu.map((item) => <a href={item.href}>{item.title}</a>)}</nav><Hello/></header>`,
			want: want{
				code: fmt.Sprintf(`<html><head></head><body><header><nav>${menu.map((item) => $$render%s<a${$$addAttribute(item.href, "href")}>${item.title}</a>%s)}</nav>${$$renderComponent($$result,'Hello',Hello,{})}</header></body></html>`, BACKTICK, BACKTICK),
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
				code: "<html><head></head><body><!-- \\`npm install astro\\` --></body></html>",
			},
		},
		{
			name:   "nested expressions",
			source: `<article>{(previous || next) && <aside>{previous && <div>Previous Article: <a rel="prev" href={new URL(previous.link, Astro.site).pathname}>{previous.text}</a></div>}{next && <div>Next Article: <a rel="next" href={new URL(next.link, Astro.site).pathname}>{next.text}</a></div>}</aside>}</article>`,
			want: want{
				code: `<html><head></head><body><article>${(previous || next) && $$render` + BACKTICK + `<aside>${previous && $$render` + BACKTICK + `<div>Previous Article: <a rel="prev"${$$addAttribute(new URL(previous.link, Astro.site).pathname, "href")}>${previous.text}</a></div>` + BACKTICK + `}${next && $$render` + BACKTICK + `<div>Next Article: <a rel="next"${$$addAttribute(new URL(next.link, Astro.site).pathname, "href")}>${next.text}</a></div>` + BACKTICK + `}</aside>` + BACKTICK + `}</article></body></html>`,
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
				code: `<html><head></head><body><div>
${
	() => {
		let generate = (input) => {
			let a = () => { return; };
			let b = () => { return; };
			let c = () => { return; };
		};
	}
}
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
				frontmatter: []string{`import Component from "test";`},
				metadata:    metadata{modules: []string{`{ module: $$module1, specifier: 'test', assert: {} }`}},
				code:        `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + "`" + `<div>Default</div>` + "`" + `,"named": () => $$render` + "`" + `<div>Named</div>` + "`" + `,})}`,
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
				code:        `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + "`" + `<div>Default</div>` + "`" + `,"named": () => $$render` + "`" + `<div>Named</div>` + "`" + `,})}`,
			},
		},
		{
			name: "slots (expression)",
			source: `
<Component {data}>
	{items.map(item => <div>{item}</div>)}
</Component>`,
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{"data":(data)},{"default": () => $$render` + BACKTICK + `${items.map(item => $$render` + BACKTICK + `<div>${item}</div>` + BACKTICK + `)}` + BACKTICK + `,})}`,
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
				styles: []string{"{props:{\"data-astro-id\":\"DPOHFLYM\"},children:`.title.astro-DPOHFLYM{font-family:fantasy;font-size:28px;}.body.astro-DPOHFLYM{font-size:1em;}`}"},
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
				frontmatter: []string{`// Component Imports
import Counter from '../components/Counter.jsx'`,
					`const someProps = {
  count: 0,
}

// Full Astro Component Syntax:
// https://docs.astro.build/core-concepts/astro-components/`},
				styles: []string{fmt.Sprintf(`{props:{"data-astro-id":"HMNNHVCQ"},children:%s:root{font-family:system-ui;padding:2em 0;}.counter{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));place-items:center;font-size:2em;margin-top:2em;}.children{display:grid;place-items:center;margin-bottom:2em;}%s}`, BACKTICK, BACKTICK)},
				metadata: metadata{
					modules:             []string{`{ module: $$module1, specifier: '../components/Counter.jsx', assert: {} }`},
					hydratedComponents:  []string{`Counter`},
					hydrationDirectives: []string{"visible"},
				},
				code: `<html lang="en" class="astro-HMNNHVCQ">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width">
    <link rel="icon" type="image/x-icon" href="/favicon.ico">

  </head>
  <body>
    <main class="astro-HMNNHVCQ">
      ${$$renderComponent($$result,'Counter',Counter,{...(someProps),"client:visible":true,"client:component-hydration":"visible","client:component-path":($$metadata.getPath(Counter)),"client:component-export":($$metadata.getExport(Counter)),"class":"astro-HMNNHVCQ"},{"default": () => $$render` + "`" + `<h1 class="astro-HMNNHVCQ">Hello React!</h1>` + "`" + `,})}
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
				frontmatter: []string{`import Widget from '../components/Widget.astro';
import Widget2 from '../components/Widget2.astro';`},
				styles: []string{},
				metadata: metadata{
					modules: []string{
						`{ module: $$module1, specifier: '../components/Widget.astro', assert: {} }`,
						`{ module: $$module2, specifier: '../components/Widget2.astro', assert: {} }`},
				},
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
				frontmatter: []string{""},
				styles:      []string{},
				scripts:     []string{fmt.Sprintf(`{props:{"type":"module","hoist":true},children:%sconsole.log("Hello");%s}`, BACKTICK, BACKTICK)},
				metadata:    metadata{hoisted: []string{fmt.Sprintf(`{ type: 'inline', value: %sconsole.log("Hello");%s }`, BACKTICK, BACKTICK)}},
				code:        `<html><head></head><body></body></html>`,
			},
		},
		{
			name: "script hoist remote",
			source: `---
---
<script type="module" hoist src="url" />`,
			want: want{
				frontmatter: []string{"\n"},
				styles:      []string{},
				scripts:     []string{`{props:{"type":"module","hoist":true,"src":"url"}}`},
				metadata:    metadata{hoisted: []string{`{ type: 'remote', src: 'url' }`}},
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
				styles:   []string{},
				scripts:  []string{"{props:{\"type\":\"module\",\"hoist\":true},children:`console.log(\"Hello\");`}"},
				metadata: metadata{hoisted: []string{fmt.Sprintf(`{ type: 'inline', value: %sconsole.log("Hello");%s }`, BACKTICK, BACKTICK)}},
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
				code: `<html><head><title>a ${expr} b</title></head><body></body></html>`,
			},
		},
		{
			name:   "text after title expressions",
			source: `<title>a {expr} b {expr} c</title>`,
			want: want{
				code: `<html><head><title>a ${expr} b ${expr} c</title></head><body></body></html>`,
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
				styles:      []string{},
				metadata:    metadata{modules: []string{`{ module: $$module1, specifier: 'test', assert: {} }`}},
				code:        `${$$renderComponent($$result,'Component',Component,{},{[name]: () => $$render` + "`" + `<div>Named</div>` + "`" + `,})}`,
			},
		},
		{
			name:   "condition expressions at the top-level",
			source: `{cond && <span></span>}{cond && <strong></strong>}`,
			want: want{
				code: "<html><head></head><body>${cond && $$render`<span></span>`}${cond && $$render`<strong></strong>`}</body></html>",
			},
		},
		{
			name:   "condition expressions at the top-level with head content",
			source: `{cond && <meta charset=utf8>}{cond && <title>My title</title>}`,
			want: want{
				code: "<html><head>${cond && $$render`<meta charset=\"utf8\">`}${cond && $$render`<title>My title</title>`}</head><body></body></html>",
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
				styles:      []string{},
				metadata:    metadata{modules: []string{`{ module: $$module1, specifier: 'test', assert: {} }`}},
				code:        `<html><head></head><body>${$$renderComponent($$result,'my-element','my-element',{})}</body></html>`,
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
				code: `${$$renderComponent($$result,'One',One,{"client:load":true,"client:component-hydration":"load","client:component-path":($$metadata.getPath(One)),"client:component-export":($$metadata.getExport(One))})}
${$$renderComponent($$result,'Two',Two,{"client:load":true,"client:component-hydration":"load","client:component-path":($$metadata.getPath(Two)),"client:component-export":($$metadata.getExport(Two))})}
${$$renderComponent($$result,'my-element','my-element',{"client:load":true,"client:component-hydration":"load","client:component-path":($$metadata.getPath('my-element')),"client:component-export":($$metadata.getExport('my-element'))})}`,
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
			source: `<html><head><script /></head><html>`,
			want: want{
				code: `<html><head><script></script></head><body></body></html>`,
			},
		},
		{
			name:   "Self-closing components in head can have siblings",
			source: `<html><head><BaseHead /><link href="test"></head><html>`,
			want: want{
				code: `<html><head>${$$renderComponent($$result,'BaseHead',BaseHead,{})}<link href="test"></head><body></body></html>`,
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
				frontmatter: []string{
					`import AComponent from '../components/AComponent.jsx';
import ZComponent from '../components/ZComponent.jsx';`},
				metadata: metadata{
					modules: []string{
						`{ module: $$module1, specifier: '../components/AComponent.jsx', assert: {} }`,
						`{ module: $$module2, specifier: '../components/ZComponent.jsx', assert: {} }`,
					},
				},
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
				code: `<html><head></head><body>` + longRandomString + `<img width="1600" height="1131" class="img" src="https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1200&q=75" srcSet="https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1200&q=75 800w,https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1200&q=75 1200w,https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=1600&q=75 1600w,https://images.unsplash.com/photo-1469854523086-cc02fe5d8800?w=2400&q=75 2400w" sizes="(max-width: 800px) 800px, (max-width: 1200px) 1200px, (max-width: 1600px) 1600px, (max-width: 2400px) 2400px, 1200px"></body></html>`,
			},
		},
		{
			name:   "SVG styles",
			source: `<svg><style>path { fill: red; }</style></svg>`,
			want: want{
				code: `<html><head></head><body><svg><style>path { fill: red; }</style></svg></body></html>`,
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
				code:        `<html><head></head><body><svg>${title ?? null}</svg></body></html>`,
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
				code:        `<html><head></head><body><svg>${title ? $$render` + BACKTICK + `<title>${title}</title>` + BACKTICK + ` : null}</svg></body></html>`,
			},
		},
		{
			name:   "Empty script",
			source: `<script hoist></script>`,
			want: want{
				scripts: []string{`{props:{"hoist":true}}`},
				code:    `<html><head></head><body></body></html>`,
			},
		},
		{
			name:   "Empty style",
			source: `<style define:vars={{ color: "Gainsboro" }}></style>`,
			want: want{
				styles: []string{`{props:{"define:vars":({ color: "Gainsboro" }),"data-astro-id":"7HAAVZPE"}}`},
				code:   `<html class="astro-7HAAVZPE"><head></head><body></body></html>`,
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
			want: want{
				code: `<!-- Global Metadata --><html><head><meta charset="utf-8">
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
</script> --></head><body></body></html>`,
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
				code:        "${$$renderComponent($$result,'Container',Container,{},{\"default\": () => $$render`${$$renderComponent($$result,'Row',Row,{},{\"default\": () => $$render`${$$renderComponent($$result,'Col',Col,{},{\"default\": () => $$render`<h1>Hi!</h1>`,})}`,})}`,})}",
			},
		},
		{
			name: "Mixed style siblings",
			source: `<head>
	<style global>div { color: red }</style>
	<style>div { color: green }</style>
	<style>div { color: blue }</style>
</head>
<div />`,
			want: want{
				styles: []string{
					"{props:{\"data-astro-id\":\"EX5CHM4O\"},children:`div.astro-EX5CHM4O{color:blue;}`}",
					"{props:{\"data-astro-id\":\"EX5CHM4O\"},children:`div.astro-EX5CHM4O{color:green;}`}",
					"{props:{\"global\":true},children:`div { color: red }`}",
				},
				code: "<html class=\"astro-EX5CHM4O\"><head>\n\n\n\n\n\n\n</head>\n<body><div class=\"astro-EX5CHM4O\"></div></body></html>",
			},
		},
		{
			name:   "Fragment",
			source: `<body><Fragment><div>Default</div><div>Named</div></Fragment></body>`,
			want: want{
				code: `<html><head></head><body>${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `<div>Default</div><div>Named</div>` + BACKTICK + `,})}</body></html>`,
			},
		},
		{
			name:   "Fragment shorthand",
			source: `<body><><div>Default</div><div>Named</div></></body>`,
			want: want{
				code: `<html><head></head><body>${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `<div>Default</div><div>Named</div>` + BACKTICK + `,})}</body></html>`,
			},
		},
		{
			name:   "Fragment slotted",
			source: `<body><Component><><div>Default</div><div>Named</div></></Component></body>`,
			want: want{
				code: `<html><head></head><body>${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + BACKTICK + `${$$renderComponent($$result,'Fragment',Fragment,{},{"default": () => $$render` + BACKTICK + `<div>Default</div><div>Named</div>` + BACKTICK + `,})}` + BACKTICK + `,})}</body></html>`,
			},
		},
		{
			name:   "Fragment slotted with name",
			source: `<body><Component><Fragment slot=named><div>Default</div><div>Named</div></Fragment></Component></body>`,
			want: want{
				code: `<html><head></head><body>${$$renderComponent($$result,'Component',Component,{},{"named": () => $$render` + BACKTICK + `${$$renderComponent($$result,'Fragment',Fragment,{"slot":"named"},{"default": () => $$render` + BACKTICK + `<div>Default</div><div>Named</div>` + BACKTICK + `,})}` + BACKTICK + `,})}</body></html>`,
			},
		},
		{
			name:   "Preserve slots inside custom-element",
			source: `<body><my-element><div slot=name>Name</div><div>Default</div></my-element></body>`,
			want: want{
				code: `<html><head></head><body>${$$renderComponent($$result,'my-element','my-element',{},{"default": () => $$render` + BACKTICK + `<div slot="name">Name</div><div>Default</div>` + BACKTICK + `,})}</body></html>`,
			},
		},
		{
			name:   "Preserve namespaces",
			source: `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><rect xlink:href="#id"></svg>`,
			want: want{
				code: `<html><head></head><body><svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><rect xlink:href="#id"></rect></svg></body></html>`,
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
				code: `<!DOCTYPE html><html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Shoperoni | Buy ${product.node.title}</title>

  <link rel="icon" type="image/svg+xml" href="/favicon.svg">
  <link rel="stylesheet" href="/style/global.css">
</head>
<body>
  ${$$renderComponent($$result,'Header',Header,{})}
  <div class="product-page">
    <article>
      ${$$renderComponent($$result,'ProductPageContent',ProductPageContent,{"client:visible":true,"product":(product.node),"client:component-hydration":"visible","client:component-path":($$metadata.getPath(ProductPageContent)),"client:component-export":($$metadata.getExport(ProductPageContent))})}
    </article>
  </div>
  ${$$renderComponent($$result,'Footer',Footer,{})}
</body></html>`,
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
				skipHoist: true,
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
			name: "select option expression",
			source: `---
const value = 'test';
---
<select><option>{value}</option></select>`,
			want: want{
				frontmatter: []string{"", "const value = 'test';"},
				code:        `<html><head></head><body><select><option>${value}</option></select></body></html>`,
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
				code:        `<html><head></head><body><select>${value && $$render` + BACKTICK + `<option>${value}</option>` + BACKTICK + `}</select></body></html>`,
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
				code:        `<html><head></head><body><textarea>${value}</textarea></body></html>`,
			},
		},
		{
			name:   "textarea inside expression",
			source: `{bool && <textarea>{value}</textarea>} {!bool && <input>}`,
			want: want{
				code: `<html><head></head><body>${bool && $$render` + BACKTICK + `<textarea>${value}</textarea>` + BACKTICK + `} ${!bool && $$render` + BACKTICK + `<input>` + BACKTICK + `}</body></html>`,
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
				code:        `<html><head></head><body><table>${items.map(item => ($$render` + BACKTICK + `<tr><td>${item}</td></tr>` + BACKTICK + `))}</table></body></html>`,
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
				code:        `<html><head></head><body><table><tbody><tr><td>Name</td></tr>${items.map(item => ($$render` + BACKTICK + `<tr><td>${item}</td></tr>` + BACKTICK + `))}</tbody></table></body></html>`,
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
				code:        `<html><head></head><body><table><tbody><tr><td>Name</td></tr>${items.map(item => ($$render` + BACKTICK + `<tr><td>${item}</td><td>${item + 's'}</td></tr>` + BACKTICK + `))}</tbody></table></body></html>`,
			},
		},
		{
			name:   "td expressions",
			source: `<table><tr><td><h2>Row 1</h2></td><td>{title}</td></tr></table>`,
			want: want{
				code: `<html><head></head><body><table><tbody><tr><td><h2>Row 1</h2></td><td>${title}</td></tr></tbody></table></body></html>`,
			},
		},
		{
			name:   "th expressions",
			source: `<table><thead><tr><th>{title}</th></tr></thead></table>`,
			want: want{
				code: `<html><head></head><body><table><thead><tr><th>${title}</th></tr></thead></table></body></html>`,
			},
		},
		{
			name:   "anchor expressions",
			source: `<a>{expr}</a>`,
			want: want{
				code: `<html><head></head><body><a>${expr}</a></body></html>`,
			},
		},
		{
			name:   "anchor inside expression",
			source: `{true && <a>expr</a>}`,
			want: want{
				code: `<html><head></head><body>${true && $$render` + BACKTICK + `<a>expr</a>` + BACKTICK + `}</body></html>`,
			},
		},
		{
			name:   "anchor content",
			source: `<a><div><h3></h3><ul><li>{expr}</li></ul></div></a>`,
			want: want{
				code: `<html><head></head><body><a><div><h3></h3><ul><li>${expr}</li></ul></div></a></body></html>`,
			},
		},
		{
			name:   "small expression",
			source: `<div><small>{a}</small>{data.map(a => <Component value={a} />)}</div>`,
			want: want{
				code: `<html><head></head><body><div><small>${a}</small>${data.map(a => $$render` + BACKTICK + `${$$renderComponent($$result,'Component',Component,{"value":(a)})}` + BACKTICK + `)}</div></body></html>`,
			},
		},
		{
			name:   "escaped entity",
			source: `<img alt="A person saying &#x22;hello&#x22;">`,
			want: want{
				code: `<html><head></head><body><img alt="A person saying &quot;hello&quot;"></body></html>`,
			},
		},
		{
			name:   "textarea in form",
			source: `<html><Component><form><textarea></textarea></form></Component></html>`,
			want: want{
				code: `<html>${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + BACKTICK + `<form><textarea></textarea></form>` + BACKTICK + `,})}</html>`,
			},
		},
		{
			name:   "slot inside of Base",
			source: `<Base title="Home"><div>Hello</div></Base>`,
			want: want{
				code: `${$$renderComponent($$result,'Base',Base,{"title":"Home"},{"default": () => $$render` + BACKTICK + `<div>Hello</div>` + BACKTICK + `,})}`,
			},
		},
		{
			name:   "user-defined `implicit` is printed",
			source: `<html implicit></html>`,
			want: want{
				code: `<html implicit><head></head><body></body></html>`,
			},
		},
		{
			name: "css comment doesn’t produce semicolon",
			source: `<style>/* comment */.container {
    padding: 2rem;
	}
</style>

<div class="container">My Text</div>`,

			want: want{
				styles: []string{fmt.Sprintf(`{props:{"data-astro-id":"RN5ULUD7"},children:%s/* comment */.container.astro-RN5ULUD7{padding:2rem;}%s}`, BACKTICK, BACKTICK)},
				code: `<html class="astro-RN5ULUD7"><head>

</head><body><div class="container astro-RN5ULUD7">My Text</div></body></html>`,
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
				code: fmt.Sprintf(`<html><head></head><body>
  <table>
  ${true ? ($$render%s<tr><td>Row 1</td></tr>%s) : null}
  ${true ? ($$render%s<tr><td>Row 2</td></tr>%s) : null}
  ${true ? ($$render%s<tr><td>Row 3</td></tr>%s) : null}

</table></body></html>`, BACKTICK, BACKTICK, BACKTICK, BACKTICK, BACKTICK, BACKTICK),
			},
		},
		{
			name:   "XElement",
			source: `<XElement {...attrs}></XElement>{onLoadString ? <script></script> : null }`,
			want: want{
				code: fmt.Sprintf(`${$$renderComponent($$result,'XElement',XElement,{...(attrs)})}${onLoadString ? $$render%s<script></script>%s : null }`, BACKTICK, BACKTICK),
			},
		},
		{
			name:   "Empty expression",
			source: "<body>({})</body>",
			want: want{
				code: `<html><head></head><body>(${(void 0)})</body></html>`,
			},
		},
		{
			name:   "Empty attribute expression",
			source: "<body attr={}></body>",
			want: want{
				code: `<html><head></head><body${$$addAttribute((void 0), "attr")}></body></html>`,
			},
		},
		{
			name:   "set:html",
			source: "<article set:html={content} />",
			want: want{
				code: `<html><head></head><body><article>${$$unescapeHTML(content)}</article></body></html>`,
			},
		},
		{
			name:   "set:text",
			source: "<article set:text={content} />",
			want: want{
				code: `<html><head></head><body><article>${$$escapeHTML(content)}</article></body></html>`,
			},
		},
		{
			name:   "set:html on Component",
			source: "<Component set:html={content} />",
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + "`${$$unescapeHTML(content)}`," + `})}`,
			},
		},
		{
			name:   "set:text on Component",
			source: "<Component set:text={content} />",
			want: want{
				code: `${$$renderComponent($$result,'Component',Component,{},{"default": () => $$render` + "`${$$escapeHTML(content)}`," + `})}`,
			},
		},
		{
			name:   "set:html on custom-element",
			source: "<custom-element set:html={content} />",
			want: want{
				code: `<html><head></head><body>${$$renderComponent($$result,'custom-element','custom-element',{},{"default": () => $$render` + "`${$$unescapeHTML(content)}`," + `})}</body></html>`,
			},
		},
		{
			name:   "set:text on custom-element",
			source: "<custom-element set:text={content} />",
			want: want{
				code: `<html><head></head><body>${$$renderComponent($$result,'custom-element','custom-element',{},{"default": () => $$render` + "`${$$escapeHTML(content)}`," + `})}</body></html>`,
			},
		},
		{
			name:   "set:html on self-closing tag",
			source: "<article set:html={content} />",
			want: want{
				code: `<html><head></head><body><article>${$$unescapeHTML(content)}</article></body></html>`,
			},
		},
		{
			name:   "set:html with other attributes",
			source: "<article set:html={content} cool=\"true\" />",
			want: want{
				code: `<html><head></head><body><article cool="true">${$$unescapeHTML(content)}</article></body></html>`,
			},
		},
		{
			name:   "set:html on empty tag",
			source: "<article set:html={content}></article>",
			want: want{
				code: `<html><head></head><body><article>${$$unescapeHTML(content)}</article></body></html>`,
			},
		},
		{
			// If both "set:*" directives are passed, we only respect the first one
			name:   "set:html and set:text",
			source: "<article set:html={content} set:text={content} />",
			want: want{
				code: `<html><head></head><body><article>${$$unescapeHTML(content)}</article></body></html>`,
			},
		},
		{
			name:   "set:html on tag with children",
			source: "<article set:html={content}>!!!</article>",
			want: want{
				code: `<html><head></head><body><article>${$$unescapeHTML(content)}</article></body></html>`,
			},
		},
		{
			name:   "set:html on tag with empty whitespace",
			source: "<article set:html={content}>   </article>",
			want: want{
				code: `<html><head></head><body><article>${$$unescapeHTML(content)}</article></body></html>`,
			},
		},
		{
			name:   "set:html on script",
			source: "<script set:html={content} />",
			want: want{
				code: `<html><head><script>${$$unescapeHTML(content)}</script></head><body></body></html>`,
			},
		},
		{
			name:   "set:html on style",
			source: "<style set:html={content} />",
			want: want{
				code: `<html><head><style>${$$unescapeHTML(content)}</style></head><body></body></html>`,
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

			if err != nil {
				t.Error(err)
			}

			hash := astro.HashFromSource(code)
			transform.ExtractStyles(doc)
			transform.Transform(doc, transform.TransformOptions{Scope: hash}) // note: we want to test Transform in context here, but more advanced cases could be tested separately
			result := PrintToJS(code, doc, 0, transform.TransformOptions{
				Scope:            "astro-XXXX",
				Site:             "https://astro.build",
				InternalURL:      "http://localhost:3000/",
				ProjectRoot:      ".",
				StaticExtraction: false,
			})
			output := string(result.Output)

			toMatch := INTERNAL_IMPORTS
			if len(tt.want.frontmatter) > 0 {
				toMatch += test_utils.Dedent(tt.want.frontmatter[0])
			}
			// Fixes some tests where getStaticPaths appears in a different location
			if tt.want.skipHoist == true && len(tt.want.getStaticPaths) > 0 {
				toMatch += "\n\n"
				toMatch += strings.TrimSpace(test_utils.Dedent(tt.want.getStaticPaths)) + "\n"
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

			toMatch += "\n\n" + fmt.Sprintf("export const %s = %s(import.meta.url, %s);\n\n", METADATA, CREATE_METADATA, metadata)
			toMatch += test_utils.Dedent(CREATE_ASTRO_CALL) + "\n\n"
			if tt.want.skipHoist != true && len(tt.want.getStaticPaths) > 0 {
				toMatch += strings.TrimSpace(test_utils.Dedent(tt.want.getStaticPaths)) + "\n\n"
			}
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
			// code
			toMatch += test_utils.Dedent(fmt.Sprintf("%s%s", RETURN, tt.want.code))
			// HACK: add period to end of test to indicate significant preceding whitespace (otherwise stripped by dedent)
			if strings.HasSuffix(toMatch, ".") {
				toMatch = strings.TrimRight(toMatch, ".")
			}
			toMatch += SUFFIX

			// compare to expected string, show diff if mismatch
			if diff := test_utils.ANSIDiff(test_utils.Dedent(toMatch), test_utils.Dedent(output)); diff != "" {
				t.Error(fmt.Sprintf("mismatch (-want +got):\n%s", diff))
			}
		})
	}
}
