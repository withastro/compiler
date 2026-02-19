package transform

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/handler"
)

func transformScopingFixtures() []struct {
	name       string
	source     string
	want       string
	scopeStyle string // "attribute" | "class" | "where"
} {
	return []struct {
		name       string
		source     string
		want       string
		scopeStyle string
	}{
		{
			name: "basic",
			source: `
				<style>div { color: red }</style>
				<div />
			`,
			want: `<div class="astro-xxxxxx"></div>`,
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
			want: `<div class="astro-xxxxxx" style={$$definedVars}></div>`,
		},
		{
			name: "scoped multiple",
			source: `
				<style>div { color: red }</style>
				<style>div { color: green }</style>
				<div />
			`,
			want: `<div class="astro-xxxxxx"></div>`,
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
			want: `<div class="astro-xxxxxx"></div>`,
		},
		{
			name: "multiple scoped :global",
			source: `
				<style>:global(test-2) {}</style>
				<style>:global(test-1) {}</style>
				<div />
			`,
			want: `<div class="astro-xxxxxx"></div>`,
		},
		{
			name: "inline does not scope",
			source: `
				<style is:inline>div{}</style>
				<div />
			`,
			want: `<div></div>`,
		},
		{
			name: "attribute -> creates a new data attribute",
			source: `
				<style>.class{}</style>
				<div />
			`,
			want:       `<div data-astro-cid-xxxxxx></div>`,
			scopeStyle: "attribute",
		},
		{
			name: "attribute -> creates data attribute when there's a class",
			source: `
				<style>.font{}</style>
				<div />
			`,
			want:       `<div data-astro-cid-xxxxxx></div>`,
			scopeStyle: "attribute",
		},
		{
			name: "attribute -> creates data attribute when there's a CSS class",
			source: `
				<style>.font{}</style>
				<div />
			`,
			want:       `<div data-astro-cid-xxxxxx></div>`,
			scopeStyle: "attribute",
		},
		{
			name: "attribute -> creates data attribute when there's already a class attribute",
			source: `
				<style>.font{}</style>
				<div class="foo" />
			`,
			want:       `<div class="foo" data-astro-cid-xxxxxx></div>`,
			scopeStyle: "attribute",
		},
	}
}

func TestTransformScoping(t *testing.T) {
	tests := transformScopingFixtures()
	var b strings.Builder
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b.Reset()
			doc, err := astro.Parse(strings.NewReader(tt.source))
			if err != nil {
				t.Error(err)
			}
			var scopeStyle string
			if tt.scopeStyle == "attribute" {
				scopeStyle = "attribute"
			} else if tt.scopeStyle == "class" {
				scopeStyle = "class"
			} else {
				scopeStyle = "where"
			}
			transformOptions := TransformOptions{Scope: "xxxxxx", ScopedStyleStrategy: scopeStyle}
			ExtractStyles(doc, &transformOptions)
			Transform(doc, transformOptions, handler.NewHandler(tt.source, "/test.astro"))
			astro.PrintToSource(&b, doc.LastChild.FirstChild.NextSibling.FirstChild)
			got := b.String()
			if tt.want != got {
				t.Errorf("\nFAIL: %s\n  want: %s\n  got:  %s", tt.name, tt.want, got)
			}
		})
	}
}

func FuzzTransformScoping(f *testing.F) {
	tests := transformScopingFixtures()
	for _, tt := range tests {
		f.Add(tt.source) // Use f.Add to provide a seed corpus
	}
	f.Fuzz(func(t *testing.T, source string) {
		doc, err := astro.Parse(strings.NewReader(source))
		if err != nil {
			t.Skip("Invalid parse, skipping rest of fuzz test")
		}
		transformOptions := TransformOptions{Scope: "xxxxxx"}
		ExtractStyles(doc, &transformOptions)
		Transform(doc, transformOptions, handler.NewHandler(source, "/test.astro"))
		var b strings.Builder
		astro.PrintToSource(&b, doc.LastChild.FirstChild.NextSibling.FirstChild)
		got := b.String()
		// hacky - we only expect scoping for non global styles / non inline styles
		testRegex := regexp.MustCompile(`is:global|:global\(|is:inline|<style>\s*</style>`)
		if !testRegex.MatchString(source) && !strings.Contains(got, "astro-xxxxxx") {
			t.Errorf("HTML scoping failed to include the astro scope\n source: %q\n got: %q", source, got)
		}
		if utf8.ValidString(source) && !utf8.ValidString(got) {
			t.Errorf("HTML scoping produced invalid html string: %q", got)
		}
	})
}

func TestFullTransform(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "top-level component with leading style",
			source: `<style>:root{}</style><Component><h1>Hello world</h1></Component>`,
			want:   `<Component><h1>Hello world</h1></Component>`,
		},
		{
			name:   "top-level component with leading style body",
			source: `<style>:root{}</style><Component><div><h1>Hello world</h1></div></Component>`,
			want:   `<Component><div><h1>Hello world</h1></div></Component>`,
		},
		{
			name:   "top-level component with trailing style",
			source: `<Component><h1>Hello world</h1></Component><style>:root{}</style>`,
			want:   `<Component><h1>Hello world</h1></Component>`,
		},

		{
			name:   "Component before html I",
			source: `<Navigation /><html><body><h1>Astro</h1></body></html>`,
			want:   `<Navigation></Navigation><h1>Astro</h1>`,
		},
		{
			name:   "Component before html II",
			source: `<MainHead title={title} description={description} /><html lang="en"><body><slot /></body></html>`,
			want:   `<MainHead title={title} description={description}></MainHead><slot></slot>`,
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
			name:   "does not remove trailing siblings",
			source: `<title>Title</title><span /><Component /><span />`,
			want:   `<title>Title</title><span></span><Component></Component><span></span>`,
		},
	}
	var b strings.Builder
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b.Reset()
			doc, err := astro.Parse(strings.NewReader(tt.source))
			if err != nil {
				t.Error(err)
			}
			transformOptions := TransformOptions{}
			ExtractStyles(doc, &transformOptions)
			// Clear doc.Styles to avoid scoping behavior, we're not testing that here
			doc.Styles = make([]*astro.Node, 0)
			Transform(doc, transformOptions, handler.NewHandler(tt.source, "/test.astro"))
			astro.PrintToSource(&b, doc)
			got := strings.TrimSpace(b.String())
			if tt.want != got {
				t.Errorf("\nFAIL: %s\n  want: %s\n  got:  %s", tt.name, tt.want, got)
			}
		})
	}
}

func TestTransformTransitionAndHeadPropagationFlags(t *testing.T) {
	tests := []struct {
		name                string
		source              string
		wantTransition      bool
		wantHeadPropagation bool
	}{
		{
			name:                "server:defer only",
			source:              `<Component server:defer />`,
			wantTransition:      false,
			wantHeadPropagation: true,
		},
		{
			name:                "transition directive",
			source:              `<div transition:animate="slide"></div>`,
			wantTransition:      true,
			wantHeadPropagation: true,
		},
		{
			name:                "transition:persist-props alone does not count as transition directive",
			source:              `<Component transition:persist-props="true" />`,
			wantTransition:      false,
			wantHeadPropagation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := astro.Parse(strings.NewReader(tt.source))
			if err != nil {
				t.Fatal(err)
			}

			transformOptions := TransformOptions{}
			ExtractStyles(doc, &transformOptions)
			Transform(doc, transformOptions, handler.NewHandler(tt.source, "/test.astro"))

			if doc.Transition != tt.wantTransition {
				t.Fatalf("unexpected doc.Transition value: want %v, got %v", tt.wantTransition, doc.Transition)
			}
			if doc.HeadPropagation != tt.wantHeadPropagation {
				t.Fatalf("unexpected doc.HeadPropagation value: want %v, got %v", tt.wantHeadPropagation, doc.HeadPropagation)
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
		{
			name:   "trailing whitespace before style is removed",
			source: "<html><head></head><body><slot />\n<style>div { color: red; }</style></body></html>",
			want:   "<html><head></head><body><slot></slot></body></html>",
		},
	}

	var b strings.Builder
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b.Reset()
			doc, err := astro.Parse(strings.NewReader(tt.source))
			if err != nil {
				t.Error(err)
			}
			transformOptions := TransformOptions{}
			ExtractStyles(doc, &transformOptions)
			// Clear doc.Styles to avoid scoping behavior, we're not testing that here
			doc.Styles = make([]*astro.Node, 0)
			Transform(doc, transformOptions, handler.NewHandler(tt.source, "/test.astro"))
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
			source: `<head>  <script>console.log("test")</script>  <head>`,
			want:   `<head><script>console.log("test")</script></head>`,
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
			name:   "collapse in-between inline elements",
			source: "<div>Click   <a>here</a> <span>space</span></div>",
			want:   "<div>Click <a>here</a> <span>space</span></div>",
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
			doc, err := astro.Parse(strings.NewReader(tt.source))
			if err != nil {
				t.Error(err)
			}
			transformOptions := TransformOptions{
				Compact: true,
			}
			ExtractStyles(doc, &transformOptions)
			// Clear doc.Styles to avoid scoping behavior, we're not testing that here
			doc.Styles = make([]*astro.Node, 0)
			Transform(doc, transformOptions, &handler.Handler{})
			astro.PrintToSource(&b, doc)
			got := strings.TrimSpace(b.String())
			if tt.want != got {
				t.Errorf("\nFAIL: %s\n  want: %s\n  got:  %s", tt.name, tt.want, got)
			}
		})
	}
}

func TestAnnotation(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "basic",
			source: `<div>Hello world!</div>`,
			want:   `<div data-astro-source-file="/src/pages/index.astro">Hello world!</div>`,
		},
		{
			name:   "no components",
			source: `<Component>Hello world!</Component>`,
			want:   `<Component>Hello world!</Component>`,
		},
		{
			name:   "injects root",
			source: `<html></html>`,
			want:   `<html></html>`,
		},
	}
	var b strings.Builder
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b.Reset()
			doc, err := astro.Parse(strings.NewReader(tt.source))
			if err != nil {
				t.Error(err)
			}
			h := handler.NewHandler(tt.source, "/src/pages/index.astro")
			Transform(doc, TransformOptions{
				AnnotateSourceFile: true,
				Filename:           "/src/pages/index.astro",
				NormalizedFilename: "/src/pages/index.astro",
			}, h)
			astro.PrintToSource(&b, doc)
			got := strings.TrimSpace(b.String())
			if tt.want != got {
				t.Errorf("\nFAIL: %s\n  want: %s\n  got:  %s", tt.name, tt.want, got)
			}

		})
	}
}

func TestClassAndClassListMerging(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "Single class attribute",
			source: `<div class="astro-xxxxxx" />`,
			want:   `<div class="astro-xxxxxx"></div>`,
		},
		{
			name:   "Class attribute with parameter",
			source: "<div class={`astro-xxxxxx ${astro}`} />",
			want:   "<div class={`astro-xxxxxx ${astro}`}></div>",
		},
		{
			name:   "Single class:list attribute",
			source: `<div class:list={"astro-xxxxxx"} />`,
			want:   `<div class:list={"astro-xxxxxx"}></div>`,
		},
		{
			name:   "Merge class with empty class:list (double quotes)",
			source: `<div class="astro-xxxxxx" class:list={} />`,
			want:   `<div class:list={['astro-xxxxxx', ]}></div>`,
		},
		{
			name:   "Merge class with empty class:list (single quotes)",
			source: `<div class='astro-xxxxxx' class:list={} />`,
			want:   `<div class:list={['astro-xxxxxx', ]}></div>`,
		},
		{
			name:   "Merge class and class:list attributes (string)",
			source: `<div class="astro-xxxxxx" class:list={"astro-yyyyyy"} />`,
			want:   `<div class:list={['astro-xxxxxx', "astro-yyyyyy"]}></div>`,
		},
		{
			name:   "Merge class and class:list attributes (expression)",
			source: `<div class={"astro-xxxxxx"} class:list={"astro-yyyyyy"} />`,
			want:   `<div class:list={["astro-xxxxxx", "astro-yyyyyy"]}></div>`,
		},
		{
			name:   "Merge Class and Class List Attributes (concatenation)",
			source: `<div class={"astro-xxxxxx" + name} class:list={"astro-yyyyyy"} />`,
			want:   `<div class:list={["astro-xxxxxx" + name, "astro-yyyyyy"]}></div>`,
		},
	}

	var b strings.Builder
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b.Reset()
			doc, err := astro.Parse(strings.NewReader(tt.source))
			if err != nil {
				t.Error(err)
			}
			Transform(doc, TransformOptions{}, handler.NewHandler(tt.source, "/test.astro"))
			astro.PrintToSource(&b, doc.LastChild.FirstChild.NextSibling.FirstChild)
			got := b.String()
			if tt.want != got {
				t.Errorf("\nFAIL: %s\n  want: %s\n  got:  %s", tt.name, tt.want, got)
			}
		})
	}
}
