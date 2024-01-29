package printer

import (
	"strings"
	"testing"

	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/handler"
	"github.com/withastro/compiler/internal/test_utils"
	"github.com/withastro/compiler/internal/transform"
	"github.com/withastro/compiler/ts_parser"
)

type testcase_css struct {
	name                string
	source              string
	want                string
	scopedStyleStrategy string
}

func TestPrinterCSS(t *testing.T) {
	tests := []testcase_css{
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
			want: ".title:where(.astro-dpohflym){font-family:fantasy;font-size:28px}.body:where(.astro-dpohflym){font-size:1em}",
		},
		{
			name: "scopedStyleStrategy: 'class'",
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
			scopedStyleStrategy: "class",
			want:                ".title.astro-dpohflym{font-family:fantasy;font-size:28px}.body.astro-dpohflym{font-size:1em}",
		},
		{
			name: "scopedStyleStrategy: 'attribute'",
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
			scopedStyleStrategy: "attribute",
			want:                ".title[data-astro-cid-dpohflym]{font-family:fantasy;font-size:28px}.body[data-astro-cid-dpohflym]{font-size:1em}",
		},
	}

	tsParser, cleanup := ts_parser.CreateTypescripParser()
	// TODO(mk): revisit where the cleanup should be called
	defer cleanup()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// transform output from source
			code := test_utils.Dedent(tt.source)

			doc, err := astro.Parse(strings.NewReader(code), tsParser)

			if err != nil {
				t.Error(err)
			}

			scopedStyleStrategy := "where"
			if tt.scopedStyleStrategy == "class" || tt.scopedStyleStrategy == "attribute" {
				scopedStyleStrategy = tt.scopedStyleStrategy
			}

			hash := astro.HashString(code)
			transform.ExtractStyles(doc)
			transform.Transform(doc, transform.TransformOptions{Scope: hash, ScopedStyleStrategy: scopedStyleStrategy}, handler.NewHandler(code, "/test.astro")) // note: we want to test Transform in context here, but more advanced cases could be tested separately
			result := PrintCSS(code, doc, transform.TransformOptions{
				Scope:       "astro-XXXX",
				InternalURL: "http://localhost:3000/",
			})
			output := ""
			for _, bytes := range result.Output {
				output += string(bytes)
			}

			toMatch := tt.want

			// compare to expected string, show diff if mismatch
			if diff := test_utils.ANSIDiff(test_utils.Dedent(toMatch), test_utils.Dedent(output)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
