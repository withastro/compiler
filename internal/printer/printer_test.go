package printer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	tycho "github.com/snowpackjs/astro/internal"
	"github.com/snowpackjs/astro/internal/test_utils"
	"github.com/snowpackjs/astro/internal/transform"
)

var INTERNAL_IMPORTS = fmt.Sprintf("import {\n  %s\n} from \"%s\";\n", strings.Join([]string{
	"render as " + TEMPLATE_TAG,
	"createComponent as " + CREATE_COMPONENT,
	"renderComponent as " + RENDER_COMPONENT,
	"addAttribute as " + ADD_ATTRIBUTE,
	"spreadAttributes as " + SPREAD_ATTRIBUTES,
	"defineStyleVars as " + DEFINE_STYLE_VARS,
	"defineScriptVars as " + DEFINE_SCRIPT_VARS,
}, ",\n  "), "http://localhost:3000/")
var PRELUDE = fmt.Sprintf(`//@ts-ignore
const $$Component = %s(async ($$result, $$props, $$slots) => {
const Astro = $$result.createAstro($$props);`, CREATE_COMPONENT)
var RETURN = fmt.Sprintf("return %s%s", TEMPLATE_TAG, BACKTICK)
var SUFFIX = fmt.Sprintf("%s;", BACKTICK) + `
});
export default $$Component;`
var STYLE_PRELUDE = "const STYLES = [\n"
var STYLE_SUFFIX = "];\n$$result.styles.add(...STYLES)\n"

func TestPrinter(t *testing.T) {
	type want struct {
		imports     string
		frontmatter []string
		styles      []string
		code        string
		// TODO: add scripts & styles for testing later?
		// scripts []string
	}
	tests := []struct {
		name   string
		source string
		want   want
	}{
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
				frontmatter: []string{"const href = '/about';"},
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
					"import VueComponent from '../components/Vue.vue';",
				},
				styles: []string{},
				code: `<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    ${` + RENDER_COMPONENT + `($$result,'VueComponent',VueComponent, null, ` + TEMPLATE_TAG + BACKTICK + BACKTICK + `)}
  </body></html>`,
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
				frontmatter: []string{"const items = [0, 1, 2];"},
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
				frontmatter: []string{"const groups = [[0, 1, 2], [3, 4, 5]];"},
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
				frontmatter: []string{`const name = "world";`},
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
				styles:      []string{},
				code: `<html><head>

</head><body><h1 class="title astro-W37SZOV4">Page Title</h1>
<p class="body astro-W37SZOV4">I’m a page</p></body></html>`,
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
const someProps = {
  count: 0,
}

// Full Astro Component Syntax:
// https://docs.astro.build/core-concepts/astro-components/`},
				styles: []string{":root{font-family:system-ui;padding:2em 0;}.counter{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));place-items:center;font-size:2em;margin-top:2em;}.children{display:grid;place-items:center;margin-bottom:2em;}"},
				code: `<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width">
    <link rel="icon" type="image/x-icon" href="/favicon.ico">

  </head>
  <body>
    <main class="astro-HMNNHVCQ">
      ${$$renderComponent($$result,'Counter',Counter, {...(someProps),"client:visible":true,"class":"astro-HMNNHVCQ"}, $$render` + "`" + `
        <h1 class="astro-HMNNHVCQ">Hello React!</h1>
      ` + "`" + `)}
    </main>
  </body></html>`,
			},
		},
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
			transform.Transform(doc, transform.TransformOptions{Scope: hash}) // note: we want to test Transform in context here, but more advanced cases could be tested separately
			result := PrintToJS(code, doc, transform.TransformOptions{
				Scope:       "astro-XXXX",
				InternalURL: "http://localhost:3000/",
			})
			output := strings.TrimSpace(test_utils.Dedent(string(result.Output)))

			toMatch := INTERNAL_IMPORTS
			if len(tt.want.frontmatter) > 0 {
				toMatch = toMatch + fmt.Sprintf(strings.TrimSpace(tt.want.frontmatter[0]))
			}
			toMatch = toMatch + "\n" + PRELUDE
			if len(tt.want.frontmatter) > 1 {
				// format want
				toMatch = toMatch + fmt.Sprintf(strings.TrimSpace(tt.want.frontmatter[0]))
			}
			toMatch = toMatch + "\n"
			if len(tt.want.styles) > 0 {
				toMatch = toMatch + STYLE_PRELUDE
				for _, style := range tt.want.styles {
					toMatch = toMatch + "  `" + style + "`,\n"
				}
				toMatch = toMatch + STYLE_SUFFIX
			}
			toMatch = toMatch + fmt.Sprintf("%s%s", RETURN, tt.want.code)
			toMatch = toMatch + SUFFIX

			// compare to expected string, show diff if mismatch
			if diff := ANSIDiff(toMatch, output); diff != "" {
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
