package js_scanner

import (
	"fmt"
	"testing"
)

func TestFindRenderBody(t *testing.T) {
	// note: the tests have hashes inlined because it’s easier to read
	// note: this must be valid CSS, hence the empty "{}"
	tests := []struct {
		name   string
		source string
		want   int
	}{
		{
			name:   "basic",
			source: `const value = "test"`,
			want:   0,
		},
		{
			name: "import",
			source: `import { fn } from "package";
const b = await fetch();`,
			want: 30,
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
			want: 44,
		},
		{
			name: "import with comment",
			source: `// comment
import { fn } from "package";
const b = await fetch();`,
			want: 41,
		},
		{
			name: "import assertion",
			source: `// comment
import { fn } from "package" assert { it: 'works' };
const b = await fetch();`,
			want: 64,
		},
		{
			name: "import assertion",
			source: `// comment
import { 
	fn
} from 
	"package" 
	assert { 
		it: 'works'
	};
const b = await fetch();`,
			want: 74,
		},
		{
			name: "import/export",
			source: `import { fn } from "package";
export async fn() {}
const b = await fetch()`,
			want: 51,
		},
		{
			name: "getStaticPaths",
			source: `import { fn } from "package";
export async function getStaticPaths() {
	const content = Astro.fetchContent('**/*.md');
}
const b = await fetch()`,
			want: 121,
		},
		{
			name: "getStaticPaths with comments",
			source: `import { fn } from "package";
export async function getStaticPaths() {
	const content = Astro.fetchContent('**/*.md');
}
const b = await fetch()`,
			want: 121,
		},
		{
			name: "getStaticPaths with semicolon",
			source: `import { fn } from "package";
export async function getStaticPaths() {
	const content = Astro.fetchContent('**/*.md');
}; const b = await fetch()`,
			want: 122,
		},
		{
			name: "multiple imports",
			source: `import { a } from "a";
import { b } from "b";
import { c } from "c";
const d = await fetch()`,
			want: 69,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindRenderBody([]byte(tt.source))
			if tt.want != got {
				t.Error(fmt.Sprintf("\nFAIL: %s\n  want: %v\n  got:  %v", tt.name, tt.want, got))
				fmt.Println(tt.source[got:])
			}
		})
	}
}
