package printer

import (
	"strings"
	"testing"

	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/handler"
	"github.com/withastro/compiler/internal/test_utils"
	"github.com/withastro/compiler/internal/transform"
)

type testcase_css struct {
	name                string
	source              string
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// transform output from source
			code := test_utils.Dedent(tt.source)

			doc, err := astro.Parse(strings.NewReader(code))

			if err != nil {
				t.Error(err)
			}

			scopedStyleStrategy := "where"
			if tt.scopedStyleStrategy == "class" || tt.scopedStyleStrategy == "attribute" {
				scopedStyleStrategy = tt.scopedStyleStrategy
			}

			hash := astro.HashString(code)
			opts := transform.TransformOptions{Scope: hash, ScopedStyleStrategy: scopedStyleStrategy, ExperimentalScriptOrder: true}
			transform.ExtractStyles(doc, &opts)
			transform.Transform(doc, opts, handler.NewHandler(code, "/test.astro")) // note: we want to test Transform in context here, but more advanced cases could be tested separately
			result := PrintCSS(code, doc, transform.TransformOptions{
				Scope:       "astro-XXXX",
				InternalURL: "http://localhost:3000/",
			})
			output := ""
			for _, bytes := range result.Output {
				output += string(bytes)
			}

			test_utils.MakeSnapshot(
				&test_utils.SnapshotOptions{
					Testing:      t,
					TestCaseName: tt.name,
					Input:        code,
					Output:       output,
					Kind:         test_utils.CssOutput,
					FolderName:   "__printer_css__",
				})
		})
	}
}
