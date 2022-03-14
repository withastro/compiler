package transform

import (
	"strings"
	"testing"

	astro "github.com/withastro/compiler/internal"
	"golang.org/x/net/html/atom"
)

func TestScopeHTML(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "none",
			source: "<div />",
			want:   `<div data-astro-scope="XXXXXX"></div>`,
		},
		{
			name:   "quoted",
			source: `<div class="test" />`,
			want:   `<div class="test" data-astro-scope="XXXXXX"></div>`,
		},
		{
			name:   "quoted no trim",
			source: `<div data-astro-scope="test " />`,
			want:   `<div data-astro-scope="test  XXXXXX"></div>`,
		},
		{
			name:   "expression string",
			source: `<div data-astro-scope={"test"} />`,
			want:   `<div data-astro-scope={("test") + " XXXXXX"}></div>`,
		},
		{
			name:   "expression function",
			source: `<div data-astro-scope={clsx({ [test]: true })} />`,
			want:   `<div data-astro-scope={(clsx({ [test]: true })) + " XXXXXX"}></div>`,
		},
		{
			name:   "expression dynamic",
			source: "<div data-astro-scope={condition ? 'a' : 'b'} />",
			want:   `<div data-astro-scope={(condition ? 'a' : 'b') + " XXXXXX"}></div>`,
		},
		{
			name:   "empty",
			source: "<div data-astro-scope />",
			want:   `<div data-astro-scope="XXXXXX"></div>`,
		},
		{
			name:   "template literal",
			source: "<div data-astro-scope=`${value}` />",
			want:   "<div data-astro-scope=`${value} XXXXXX`></div>",
		},
		{
			name:   "component className not scoped",
			source: `<Component data-astro-scope="test" />`,
			want:   `<Component data-astro-scope="test XXXXXX"></Component>`,
		},
		{
			name:   "component className expression",
			source: `<Component data-astro-scope={"test"} />`,
			want:   `<Component data-astro-scope={("test") + " XXXXXX"}></Component>`,
		},
		{
			name:   "element class:list",
			source: "<div class:list={{ a: true }} />",
			want:   `<div class:list={[({ a: true }), "astro-XXXXXX"]}></div>`,
		},
		{
			name:   "element class:list string",
			source: "<div class:list=\"weird but ok\" />",
			want:   `<div class:list="weird but ok astro-XXXXXX"></div>`,
		},
		{
			name:   "component class:list",
			source: "<Component class:list={{ a: true }} />",
			want:   `<Component class:list={[({ a: true }), "astro-XXXXXX"]}></Component>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes, err := astro.ParseFragment(strings.NewReader(tt.source), &astro.Node{Type: astro.ElementNode, DataAtom: atom.Body, Data: atom.Body.String()})
			if err != nil {
				t.Error(err)
			}
			ScopeElement(nodes[0], TransformOptions{Scope: "XXXXXX"})
			var b strings.Builder
			astro.PrintToSource(&b, nodes[0])
			got := b.String()
			if tt.want != got {
				t.Errorf("\nFAIL: %s\n  want: %s\n  got:  %s", tt.name, tt.want, got)
			}
		})
	}
}
