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
			want:   -1,
		},
		{
			name: "basic split",
			source: `const a = "test"
const b = await fetch();`,
			want: 17,
		},
		{
			name:   "await",
			source: `const value = await fetch()`,
			want:   0,
		},
		{
			name:   "Astro",
			source: `const { props } = Astro`,
			want:   0,
		},
		{
			name:   "await inside async shorthand",
			source: `const value = async () => (await fetch())`,
			want:   -1,
		},
		{
			name: "await inside async longhand",
			source: `const value = async () => {
				await fetch()
			}`,
			want: -1,
		},
		{
			name: "getStaticPaths",
			source: `export async function getStaticPaths() {
				await fetch()
			}`,
			want: -1,
		},
		{
			name: "getStaticPaths Astro",
			source: `export async function getStaticPaths() {
				const content = Astro.fetchContent()
			}`,
			want: -1,
		},
		{
			name: "sync function Astro",
			source: `export function Something() {
				const content = Astro.fetchContent()
			}`,
			want: -1,
		},
		{
			name:   "sync function shorthand Astro",
			source: `export const fn = () => Astro.fetchContent()`,
			want:   -1,
		},
		{
			name:   "async function shorthand",
			source: `export const fn = async () => (await fetch())`,
			want:   -1,
		},
		{
			name: "async function shorthand split",
			source: `export const fn = async () => {
	return await fetch();
}
await fn()`,
			want: 57,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindRenderBody([]byte(tt.source))
			if tt.want != got {
				t.Error(fmt.Sprintf("\nFAIL: %s\n  want: %v\n  got:  %v", tt.name, tt.want, got))
			}
		})
	}
}
