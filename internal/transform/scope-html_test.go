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
			want:   `<div class="astro-XXXXXX"></div>`,
		},
		{
			name:   "quoted",
			source: `<div class="test" />`,
			want:   `<div class="test astro-XXXXXX"></div>`,
		},
		{
			name:   "quoted no trim",
			source: `<div class="test " />`,
			want:   `<div class="test  astro-XXXXXX"></div>`,
		},
		{
			name:   "expression string",
			source: `<div class={"test"} />`,
			want:   `<div class={("test") + " astro-XXXXXX"}></div>`,
		},
		{
			name:   "expression function",
			source: `<div class={clsx({ [test]: true })} />`,
			want:   `<div class={(clsx({ [test]: true })) + " astro-XXXXXX"}></div>`,
		},
		{
			name:   "expression dynamic",
			source: "<div class={condition ? 'a' : 'b'} />",
			want:   `<div class={(condition ? 'a' : 'b') + " astro-XXXXXX"}></div>`,
		},
		{
			name:   "empty",
			source: "<div class />",
			want:   `<div class="astro-XXXXXX"></div>`,
		},
		{
			name:   "template literal",
			source: "<div class=`${value}` />",
			want:   "<div class=`${value} astro-XXXXXX`></div>",
		},
		{
			name:   "component className not scoped",
			source: `<Component className="test" />`,
			want:   `<Component className="test astro-XXXXXX"></Component>`,
		},
		{
			name:   "component className expression",
			source: `<Component className={"test"} />`,
			want:   `<Component className={("test") + " astro-XXXXXX"}></Component>`,
		},
		{
			name:   "component className shorthand",
			source: "<Component {className} />",
			want:   `<Component className={className + " astro-XXXXXX"}></Component>`,
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
		{
			name:   "fault input currently accepted",
			source: `<A { 0>`,
			want:   `<A  0>={0>} class="astro-XXXXXX"></A>`,
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
			// check whether another pass doesn't error
			nodes, err = astro.ParseFragment(strings.NewReader(got), &astro.Node{Type: astro.ElementNode, DataAtom: atom.Body, Data: atom.Body.String()})
			if err != nil {
				t.Error(err)
			}
			ScopeElement(nodes[0], TransformOptions{Scope: "XXXXXX"})
			astro.PrintToSource(&b, nodes[0])
		})
	}
}
