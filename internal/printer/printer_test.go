package printer

import (
	"strings"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
	tycho "github.com/snowpackjs/astro/internal"
	"github.com/snowpackjs/astro/internal/test_utils"
	"github.com/snowpackjs/astro/internal/transform"
)

func TestPrinter(t *testing.T) {
	type want struct {
		code string
		// TODO: add scripts & styles for testing later?
		// scripts []string
		// styles []string
	}
	tests := []struct {
		name string
		source string
		want want
	}{
		{
			"basic (no frontmatter)",
			`<button>Click</button>`,
			want {
				`//@ts-ignore
				const Component = $$createComponent(async ($$result, $$props, $$slots) => {
return ` + "`"+ `<html><head></head><body><button>Click</button></body></html>` + "`" + `;
});

export default Component;`,
			},
		},
		{
			"basic (frontmatter)",
			`---
const href = '/about';
---
<a href={href}>About</a>`,
			want {
				`//@ts-ignore
const Component = $$createComponent(async ($$result, $$props, $$slots) => {
// ---
const href = '/about';
// ---
return ` + "`" + `<html><head></head><body><a${addAttribute(href, "href")}>About</a></body></html>` + "`" + `;
});

export default Component;`,
			},
		},
		{
			"component",
			`---
import VueComponent from '../components/Vue';
const title = 'My Page';
---
<html>
  <head>
	  <title>{title}</title>
	</head>
	<body>
	  <VueComponent />
	</body>
</html>`,
			want {
				`import VueComponent from '../components/Vue';
//@ts-ignore
const Component = $$createComponent(async ($$result, $$props, $$slots) => {
// ---
const title = 'My Page';
// ---
return ` + "`" + `<html><head>
		<title>${title}</title>
	</head>
	<body>
		${renderComponent(VueComponent, null, ` + "``" + `)}
	</body>
</html>
` + "`" + `
});

export default Component;`,
			},
		},
		{
			"styles (no frontmatter)",
			`<style>
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
			want {
				`//@ts-ignore
				const Component = $$createComponent(async ($$result, $$props, $$slots) => {
return ` + "`" + `<html><head><style data-astro-id="RV7KTNA5">.title.astro-RV7KTNA5 {font-family:fantasy;font-size:28px;}.body.astro-RV7KTNA5 {font-size:1em;}</style>

</head><body><h1 class="title astro-RV7KTNA5">Page Title</h1>
<p class="body astro-RV7KTNA5">I’m a page</p></body>
</html>` + "`" + `;
});

export default Component;`,
			},
		},
	}

	dmp := diffmatchpatch.New() // differ

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// transform output from source
			code := test_utils.Dedent(tt.source)
			doc, err := tycho.Parse(strings.NewReader(code))
			if (err != nil) {
				t.Error(err)
			}
			hash := tycho.HashFromSource(code)
			transform.Transform(doc, transform.TransformOptions{ Scope: hash }) // note: we want to test Transform in context here, but more advanced cases could be tested separately
     	result := PrintToJS(code, doc)
			output := string(result.Output)

			// compare to expected string, show diff if mismatch
			if (output != tt.want.code) {
				diffs := dmp.DiffMain(tt.want.code, output, false)
				t.Error(dmp.DiffPrettyText(diffs))
			}
		})
	}
}
