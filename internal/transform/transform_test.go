package transform

import (
	"strings"
	"testing"

	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/handler"
)

func TestTransformScoping(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "basic",
			source: `
				<style>div { color: red }</style>
				<div />
			`,
			want: `<div class="astro-XXXXXX"></div>`,
		},

		{
			name: "global empty",
			source: `
				<style is:global>div { color: red }</style>
				<div />
			`,
			want: `<div></div>`,
		},
		{
			name: "global true",
			source: `
				<style is:global={true}>div { color: red }</style>
				<div />
			`,
			want: `<div></div>`,
		},
		{
			name: "global string",
			source: `
				<style is:global="">div { color: red }</style>
				<div />
			`,
			want: `<div></div>`,
		},
		{
			name: "global string true",
			source: `
				<style is:global="true">div { color: red }</style>
				<div />
			`,
			want: `<div></div>`,
		},
		{
			name: "empty (space)",
			source: `
				<style>
				
				</style>
				<div />
			`,
			want: `<div></div>`,
		},
		{
			name: "empty (nil)",
			source: `
				<style></style>
				<div />
			`,
			want: `<div></div>`,
		},
		{
			name: "empty (define:vars)",
			source: `
				<style define:vars={{ a }}></style>
				<div />
			`,
			want: `<div class="astro-XXXXXX" style={$$definedVars}></div>`,
		},
		{
			name: "scoped multiple",
			source: `
				<style>div { color: red }</style>
				<style>div { color: green }</style>
				<div />
			`,
			want: `<div class="astro-XXXXXX"></div>`,
		},
		{
			name: "global multiple",
			source: `
				<style is:global>div { color: red }</style>
				<style is:global>div { color: green }</style>
				<div />
			`,
			want: `<div></div>`,
		},
		{
			name: "mixed multiple",
			source: `
				<style>div { color: red }</style>
				<style is:global>div { color: green }</style>
				<div />
			`,
			want: `<div class="astro-XXXXXX"></div>`,
		},
		{
			name: "multiple scoped :global",
			source: `
				<style>:global(test-2) {}</style>
				<style>:global(test-1) {}</style>
				<div />
			`,
			want: `<div class="astro-XXXXXX"></div>`,
		},
		{
			name: "inline does not scope",
			source: `
				<style is:inline>div{}</style>
				<div />
			`,
			want: `<div></div>`,
		},
	}
	var b strings.Builder
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b.Reset()
			doc, err := astro.Parse(strings.NewReader(tt.source), &handler.Handler{})
			if err != nil {
				t.Error(err)
			}
			ExtractStyles(doc)
			Transform(doc, TransformOptions{Scope: "XXXXXX"}, handler.NewHandler(tt.source, "/test.astro"))
			astro.PrintToSource(&b, doc.LastChild.FirstChild.NextSibling.FirstChild)
			got := b.String()
			if tt.want != got {
				t.Errorf("\nFAIL: %s\n  want: %s\n  got:  %s", tt.name, tt.want, got)
			}
		})
	}
}

func TestFullTransform(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "top-level component with leading style",
			source: `<style>:root{}</style><Component><h1>Hello world</h1></Component>
			`,
			want: `<Component><h1>Hello world</h1></Component>`,
		},
		{
			name: "top-level component with leading style body",
			source: `<style>:root{}</style><Component><div><h1>Hello world</h1></div></Component>
			`,
			want: `<Component><div><h1>Hello world</h1></div></Component>`,
		},
		{
			name: "top-level component with trailing style",
			source: `<Component><h1>Hello world</h1></Component><style>:root{}</style>
			`,
			want: `<Component><h1>Hello world</h1></Component>`,
		},
		{
			name:   "respects explicitly authored elements",
			source: `<html><Component /></html>`,
			want:   `<html><Component></Component></html>`,
		},
		{
			name:   "respects explicitly authored elements 2",
			source: `<head></head><Component />`,
			want:   `<head></head><Component></Component>`,
		},
		{
			name:   "respects explicitly authored elements 3",
			source: `<body><Component /></body>`,
			want:   `<body><Component></Component></body>`,
		},
		{
			name:   "removes implicitly generated elements",
			source: `<Component />`,
			want:   `<Component></Component>`,
		},
		{
			name:   "works with nested components",
			source: `<style></style><A><div><B /></div></A>`,
			want:   `<A><div><B></B></div></A>`,
		},
		{
			name: "does not remove trailing siblings",
			source: `<title>Title</title>
<span />
<Component />
<span />`,
			want: `<title>Title</title>
<span></span>
<Component></Component>
<span></span>`,
		},
	}
	var b strings.Builder
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b.Reset()
			doc, err := astro.Parse(strings.NewReader(tt.source), &handler.Handler{})
			if err != nil {
				t.Error(err)
			}
			ExtractStyles(doc)
			// Clear doc.Styles to avoid scoping behavior, we're not testing that here
			doc.Styles = make([]*astro.Node, 0)
			Transform(doc, TransformOptions{}, handler.NewHandler(tt.source, "/test.astro"))
			astro.PrintToSource(&b, doc)
			got := strings.TrimSpace(b.String())
			if tt.want != got {
				t.Errorf("\nFAIL: %s\n  want: %s\n  got:  %s", tt.name, tt.want, got)
			}
		})
	}
}

func TestTransformTrailingSpace(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "component with trailing space",
			source: "<h1>Hello world</h1>\n\n\t ",
			want:   `<h1>Hello world</h1>`,
		},
		{
			name:   "component with no trailing space",
			source: "<h1>Hello world</h1>",
			want:   "<h1>Hello world</h1>",
		},
		{
			name:   "component with leading and trailing space",
			source: "<span/>\n\n\t <h1>Hello world</h1>\n\n\t ",
			want:   "<span></span>\n\n\t <h1>Hello world</h1>",
		},
		{
			name:   "html with explicit space",
			source: "<html><body>\n\n\n</body></html>",
			want:   "<html><body>\n\n\n</body></html>",
		},
	}
	var b strings.Builder
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b.Reset()
			doc, err := astro.Parse(strings.NewReader(tt.source), &handler.Handler{})
			if err != nil {
				t.Error(err)
			}
			ExtractStyles(doc)
			// Clear doc.Styles to avoid scoping behavior, we're not testing that here
			doc.Styles = make([]*astro.Node, 0)
			Transform(doc, TransformOptions{}, handler.NewHandler(tt.source, "/test.astro"))
			astro.PrintToSource(&b, doc)
			got := b.String()
			if tt.want != got {
				t.Errorf("\nFAIL: %s\n  want: %s\n  got:  %s", tt.name, tt.want, got)
			}
		})
	}
}

func TestCompactTransform(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "trims whitespace",
			source: `<div>    Test     </div>`,
			want:   `<div> Test </div>`,
		},
		{
			name:   "pre",
			source: `<pre>  Test  </pre>`,
			want:   `<pre>  Test  </pre>`,
		},
		{
			name:   "textarea",
			source: `<textarea>  Test  </textarea>`,
			want:   `<textarea>  Test  </textarea>`,
		},
		{
			name:   "deep pre",
			source: `<pre>  <div> Test </div>  </pre>`,
			want:   `<pre>  <div> Test </div>  </pre>`,
		},
		{
			name:   "remove whitespace only",
			source: `<head>  <script>console.log("hoisted")</script>  <head>`,
			want:   `<head></head>`,
		},
		{
			name:   "collapse surrounding whitespace",
			source: `<div>  COOL  </div>`,
			want:   `<div> COOL </div>`,
		},
		{
			name:   "collapse only surrounding whitespace",
			source: `<div>  C O O L  </div>`,
			want:   `<div> C O O L </div>`,
		},
		{
			name:   "collapse surrounding newlines",
			source: "<div>\n\n\tC O O L\n\n\t</div>",
			want:   "<div>\nC O O L\n</div>",
		},
		{
			name:   "expression trim first",
			source: "<div>{\n() => {\n\t\treturn <span />}}</div>",
			want:   "<div>{() => {\n\t\treturn <span></span>}}</div>",
		},
		{
			name:   "expression trim last",
			source: "<div>{() => {\n\t\treturn <span />}\n}</div>",
			want:   "<div>{() => {\n\t\treturn <span></span>}}</div>",
		},
		{
			name:   "expression collapse inside",
			source: "<div>{() => {\n\t\treturn <span>  HEY  </span>}}</div>",
			want:   "<div>{() => {\n\t\treturn <span> HEY </span>}}</div>",
		},
		{
			name:   "expression collapse newlines",
			source: "<div>{() => {\n\t\treturn <span>\n\nTEST</span>}}</div>",
			want:   "<div>{() => {\n\t\treturn <span>\nTEST</span>}}</div>",
		},
		{
			name:   "expression remove only whitespace",
			source: "<div>{() => {\n\t\treturn <span>\n\n\n</span>}}</div>",
			want:   "<div>{() => {\n\t\treturn <span></span>}}</div>",
		},
		{
			name:   "attributes",
			source: `<div    a="1"    b={0} />`,
			want:   `<div a="1" b={0}></div>`,
		},
		{
			name:   "expression quoted",
			source: "<div test={\n`  test  `\n} />",
			want:   "<div test={`  test  `}></div>",
		},
		{
			name:   "expression attribute math",
			source: "<div test={ a + b } />",
			want:   "<div test={a + b}></div>",
		},
		{
			name:   "expression math",
			source: "<div>{ a + b }</div>",
			want:   "<div>{a + b}</div>",
		},
	}
	var b strings.Builder
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b.Reset()
			doc, err := astro.Parse(strings.NewReader(tt.source), &handler.Handler{})
			if err != nil {
				t.Error(err)
			}
			ExtractStyles(doc)
			// Clear doc.Styles to avoid scoping behavior, we're not testing that here
			doc.Styles = make([]*astro.Node, 0)
			Transform(doc, TransformOptions{
				Compact: true,
			}, &handler.Handler{})
			astro.PrintToSource(&b, doc)
			got := strings.TrimSpace(b.String())
			if tt.want != got {
				t.Errorf("\nFAIL: %s\n  want: %s\n  got:  %s", tt.name, tt.want, got)
			}
		})
	}
}
