package transform

import (
	"strings"
	"testing"

	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/test_utils"
)

func TestScopeStyle(t *testing.T) {
	// note: the tests have hashes inlined because it’s easier to read
	// note: this must be valid CSS, hence the empty "{}"
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "class",
			source: ".class{}",
			want:   ".class.astro-XXXXXX{}",
		},
		{
			name:   "id",
			source: "#class{}",
			want:   "#class.astro-XXXXXX{}",
		},
		{
			name:   "element",
			source: "h1{}",
			want:   "h1.astro-XXXXXX{}",
		},
		{
			name:   "adjacent sibling",
			source: ".class+.class{}",
			want:   ".class.astro-XXXXXX+.class.astro-XXXXXX{}",
		},
		{
			name:   "and selector",
			source: ".class,.class{}",
			want:   ".class.astro-XXXXXX,.class.astro-XXXXXX{}",
		},
		{
			name:   "children universal",
			source: ".class *{}",
			want:   ".class.astro-XXXXXX .astro-XXXXXX{}",
		},
		{
			name:   "attr",
			source: "a[aria-current=\"page\"]{}",
			want:   "a.astro-XXXXXX[aria-current=page]{}",
		},
		{
			name:   "attr universal implied",
			source: "[aria-visible],[aria-hidden]{}",
			want:   ".astro-XXXXXX[aria-visible],.astro-XXXXXX[aria-hidden]{}",
		},
		{
			name:   "universal pseudo state",
			source: "*:hover{}",
			want:   ".astro-XXXXXX:hover{}",
		},
		{
			name:   "immediate child universal",
			source: ".class>*{}",
			want:   ".class.astro-XXXXXX>.astro-XXXXXX{}",
		},
		{
			name:   "element + pseudo state",
			source: ".class button:focus{}",
			want:   ".class.astro-XXXXXX button.astro-XXXXXX:focus{}",
		},
		{
			name:   "element + pseudo element",
			source: ".class h3::before{}",
			want:   ".class.astro-XXXXXX h3.astro-XXXXXX::before{}",
		},
		{
			name:   "media query",
			source: "@media screen and (min-width:640px){.class{}}",
			want:   "@media screen and (min-width:640px){.class.astro-XXXXXX{}}",
		},
		{
			name:   "element + pseudo state + pseudo element",
			source: "button:focus::before{}",
			want:   "button.astro-XXXXXX:focus::before{}",
		},
		{
			name:   "global children",
			source: ".class :global(ul li){}",
			want:   ".class.astro-XXXXXX ul li{}",
		},
		{
			name:   "global universal",
			source: ".class :global(*){}",
			want:   ".class.astro-XXXXXX *{}",
		},
		{
			name:   "global with scoped children",
			source: ":global(section) .class{}",
			want:   "section .class.astro-XXXXXX{}",
		},
		{
			name:   "subsequent siblings + global",
			source: ".class~:global(a){}",
			want:   ".class.astro-XXXXXX~a{}",
		},
		{
			name:   "global nested parens",
			source: ".class :global(.nav:not(.is-active)){}",
			want:   ".class.astro-XXXXXX .nav:not(.is-active){}",
		},
		{
			name:   "global nested parens + chained class",
			source: ":global(body:not(.is-light)).is-dark,:global(body:not(.is-dark)).is-light{}",
			want:   "body:not(.is-light).is-dark,body:not(.is-dark).is-light{}",
		},
		{
			name:   "global chaining global",
			source: ":global(.foo):global(.bar){}",
			want:   ".foo.bar{}",
		},
		{
			name:   "class chained global",
			source: ".class:global(.bar){}",
			want:   ".class.astro-XXXXXX.bar{}", // technically this may be incorrect, but would require a lookahead to fix
		},
		{
			name:   "chained :not()",
			source: ".class:not(.is-active):not(.is-disabled){}",
			want:   ".class.astro-XXXXXX:not(.is-active):not(.is-disabled){}",
		},
		{
			name:   "weird chaining",
			source: ":hover.a:focus{}", // yes this is valid. yes I’m just upset as you are :(
			want:   ":hover.a.astro-XXXXXX:focus{}",
		},
		{
			name:   "more weird chaining",
			source: ":not(.is-disabled).a{}",
			want:   ":not(.is-disabled).a.astro-XXXXXX{}",
		},
		{
			name:   "body",
			source: "body h1{}",
			want:   "body h1.astro-XXXXXX{}",
		},
		{
			name:   "body class",
			source: "body.theme-dark{}",
			want:   "body.theme-dark{}",
		},
		{
			name:   "html and body",
			source: "html,body{}",
			want:   "html,body{}",
		},
		{
			name:   ":root",
			source: ":root{}",
			want:   ":root{}",
		},
		{
			name:   "escaped characters",
			source: ".class\\:class:focus{}",
			want:   ".class\\:class.astro-XXXXXX:focus{}",
		},
		// the following tests assert we leave valid CSS alone
		{
			name:   "attributes",
			source: "body{background-image:url('/assets/bg.jpg');clip-path:polygon(0% 0%,100% 0%,100% 100%,0% 100%);}",
			want:   "body{background-image:url(/assets/bg.jpg);clip-path:polygon(0% 0%,100% 0%,100% 100%,0% 100%)}",
		},
		{
			name:   "variables",
			source: "body{--bg:red;background:var(--bg);color:black;}",
			want:   "body{--bg:red;background:var(--bg);color:black}",
		},
		{
			name:   "keyframes",
			source: "@keyframes shuffle{from{transform:rotate(0deg);}to{transform:rotate(360deg);}}",
			want:   "@keyframes shuffle{from{transform:rotate(0deg)}to{transform:rotate(360deg)}}",
		},
		{
			name:   "keyframes 2",
			source: "@keyframes shuffle{0%{transform:rotate(0deg);color:blue}100%{transform:rotate(360deg)}}",
			want:   "@keyframes shuffle{0%{transform:rotate(0deg);color:blue}100%{transform:rotate(360deg)}}",
		},
		{
			name:   "keyframes start",
			source: "@keyframes shuffle{0%{transform:rotate(0deg);color:blue}100%{transform:rotate(360deg)}} h1{} h2{}",
			want:   "@keyframes shuffle{0%{transform:rotate(0deg);color:blue}100%{transform:rotate(360deg)}}h1.astro-XXXXXX{}h2.astro-XXXXXX{}",
		},
		{
			name:   "keyframes middle",
			source: "h1{} @keyframes shuffle{0%{transform:rotate(0deg);color:blue}100%{transform:rotate(360deg)}} h2{}",
			want:   "h1.astro-XXXXXX{}@keyframes shuffle{0%{transform:rotate(0deg);color:blue}100%{transform:rotate(360deg)}}h2.astro-XXXXXX{}",
		},
		{
			name:   "keyframes end",
			source: "h1{} h2{} @keyframes shuffle{0%{transform:rotate(0deg);color:blue}100%{transform:rotate(360deg)}}",
			want:   "h1.astro-XXXXXX{}h2.astro-XXXXXX{}@keyframes shuffle{0%{transform:rotate(0deg);color:blue}100%{transform:rotate(360deg)}}",
		},
		{
			name:   "calc",
			source: ":root{padding:calc(var(--space) * 2);}",
			want:   ":root{padding:calc(var(--space) * 2)}",
		},
		{
			name:   "grid-template-columns",
			source: "div{grid-template-columns: [content-start] 1fr [content-end];}",
			want:   "div.astro-XXXXXX{grid-template-columns:[content-start] 1fr [content-end]}",
		},
		{
			name:   "charset",
			source: "@charset \"utf-8\";",
			want:   "@charset \"utf-8\";",
		},
		{
			name:   "import (plain)",
			source: "@import \"./my-file.css\";",
			want:   "@import\"./my-file.css\";",
		},
		{
			name:   "import (url)",
			source: "@import url(\"./my-file.css\");",
			want:   "@import\"./my-file.css\";",
		},
		{
			name:   "valid CSS, madeup syntax",
			source: "@tailwind base;",
			want:   "@tailwind base;",
		},
		{
			name: "invalid CSS (missing semi)",
			source: `.foo {
  color: blue
  font-size: 18px;
}`,
			want: `.foo.astro-XXXXXX{color:blue font-size: 18px}`,
		},
		{
			name:   "nesting media",
			source: ":global(html) { @media (min-width: 640px) { color: blue } }html { background-color: lime }",
			want:   "html{@media (min-width: 640px){color:blue}}html{background-color:lime}",
		},
		{
			name:   "nesting combinator",
			source: "div { & span { color: blue } }",
			want:   "div.astro-XXXXXX{& span.astro-XXXXXX{color:blue}}",
		},
		{
			name:   "nesting modifier",
			source: ".header { background-color: white; &.dark { background-color: blue; }}",
			want:   ".header.astro-XXXXXX{background-color:white;&.dark{background-color:blue}}",
		},
		{
			name: "@container",
			source: `@container (min-width: 200px) and (min-height: 200px) {
        h1 {
          font-size: 30px;
        }
      }`,
			want: "@container (min-width: 200px) and (min-height: 200px){h1.astro-XXXXXX{font-size:30px}}",
		},
		{
			name:   "@layer",
			source: "@layer theme, layout, utilities; @layer special { .item { color: rebeccapurple; }}",
			want:   "@layer theme,layout,utilities;@layer special{.item.astro-XXXXXX{color:rebeccapurple}}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// note: the "{}" is only added to make it valid CSS
			code := test_utils.Dedent("<style>\n" + tt.source + " \n</style>")
			doc, err := astro.Parse(strings.NewReader(code))
			if err != nil {
				t.Error(err)
			}
			styleEl := doc.LastChild.FirstChild.FirstChild // note: root is <html>, and we need to get <style> which lives in head
			styles := []*astro.Node{styleEl}
			ScopeStyle(styles, TransformOptions{Scope: "XXXXXX"})
			got := styles[0].FirstChild.Data
			if tt.want != got {
				t.Errorf("\nFAIL: %s\n  want: %s\n  got:  %s", tt.name, tt.want, got)
			}
		})
	}
}
