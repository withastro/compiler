package js_scanner

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type testcase struct {
	name   string
	source string
	want   string
	only   bool
}

func TestFindRenderBody(t *testing.T) {
	// note: the tests have hashes inlined because it’s easier to read
	// note: this must be valid CSS, hence the empty "{}"
	tests := []testcase{
		{
			name:   "basic",
			source: `const value = "test"`,
			want:   ``,
		},
		{
			name: "import",
			source: `import { fn } from "package";
const b = await fetch();`,
			want: `import { fn } from "package";
`,
		},
		{
			name: "big import",
			source: `import {
  a,
  b,
  c,
  d,
} from "package"

const b = await fetch();`,
			want: `import {
  a,
  b,
  c,
  d,
} from "package"

`,
		},
		{
			name: "import with comment",
			source: `// comment
import { fn } from "package";
const b = await fetch();`,
			want: `// comment
import { fn } from "package";
`,
		},
		{
			name: "import assertion",
			source: `// comment
import { fn } from "package" assert { it: 'works' };
const b = await fetch();`,
			want: `// comment
import { fn } from "package" assert { it: 'works' };
`,
		},
		{
			name: "import assertion 2",
			source: `// comment
import {
  fn
} from
  "package"
  assert {
    it: 'works'
  };
const b = await fetch();`,
			want: `// comment
import {
  fn
} from
  "package"
  assert {
    it: 'works'
  };
`,
		},
		{
			name: "import/export",
			source: `import { fn } from "package";
export async fn() {}
const b = await fetch()`,
			want: `import { fn } from "package";
export async fn() {}
`,
		},
		{
			name: "getStaticPaths",
			source: `import { fn } from "package";
export async function getStaticPaths() {
	const content = Astro.fetchContent('**/*.md');
}
const b = await fetch()`,
			want: `import { fn } from "package";
export async function getStaticPaths() {
	const content = Astro.fetchContent('**/*.md');
}
`,
		},
		{
			name: "getStaticPaths with comments",
			source: `import { fn } from "package";
export async function getStaticPaths() {
  const content = Astro.fetchContent('**/*.md');
}
const b = await fetch()`,
			want: `import { fn } from "package";
export async function getStaticPaths() {
  const content = Astro.fetchContent('**/*.md');
}
`,
		},
		{
			name: "getStaticPaths with semicolon",
			source: `import { fn } from "package";
export async function getStaticPaths() {
  const content = Astro.fetchContent('**/*.md');
}; const b = await fetch()`,
			want: `import { fn } from "package";
export async function getStaticPaths() {
  const content = Astro.fetchContent('**/*.md');
}; `,
		},
		{
			name: "multiple imports",
			source: `import { a } from "a";
import { b } from "b";
import { c } from "c";
const d = await fetch()`,
			want: `import { a } from "a";
import { b } from "b";
import { c } from "c";
`,
		},
		{
			name:   "assignment",
			source: `let show = true;`,
			want:   ``,
		},
		{
			name: "RegExp is not a comment",
			source: `import { a } from "a";
/import { b } from "b";
import { c } from "c";`,
			want: `import { a } from "a";
`,
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
			split := FindRenderBody([]byte(tt.source))
			got := tt.source[:split]
			// compare to expected string, show diff if mismatch
			if diff := ANSIDiff(got, tt.want); diff != "" {
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
