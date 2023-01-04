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
	name   string
	source string
	want   string
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
			want: ".title:where(.astro-DPOHFLYM){font-family:fantasy;font-size:28px}.body:where(.astro-DPOHFLYM){font-size:1em}",
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

			hash := astro.HashFromSource(code)
			transform.ExtractStyles(doc)
			transform.Transform(doc, transform.TransformOptions{Scope: hash}, handler.NewHandler(code, "/test.astro")) // note: we want to test Transform in context here, but more advanced cases could be tested separately
			result := PrintCSS(code, doc, transform.TransformOptions{
				Scope:       "astro-XXXX",
				Site:        "https://astro.build",
				InternalURL: "http://localhost:3000/",
				ProjectRoot: ".",
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
