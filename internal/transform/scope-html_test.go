package transform

import (
	"strings"
	"testing"
	"unicode/utf8"

	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/handler"
	"golang.org/x/net/html/atom"
)

func tests() []struct {
	name   string
	source string
	want   string
} {
	return []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "none",
			source: "<div />",
			want:   `<div class="astro-xxxxxx"></div>`,
		},
		{
			name:   "quoted",
			source: `<div class="test" />`,
			want:   `<div class="test astro-xxxxxx"></div>`,
		},
		{
			name:   "quoted no trim",
			source: `<div class="test " />`,
			want:   `<div class="test  astro-xxxxxx"></div>`,
		},
		{
			name:   "expression string",
			source: `<div class={"test"} />`,
			want:   `<div class={(("test") ?? "") + " astro-xxxxxx"}></div>`,
		},
		{
			name:   "expression function",
			source: `<div class={clsx({ [test]: true })} />`,
			want:   `<div class={((clsx({ [test]: true })) ?? "") + " astro-xxxxxx"}></div>`,
		},
		{
			name:   "expression dynamic",
			source: "<div class={condition ? 'a' : 'b'} />",
			want:   `<div class={((condition ? 'a' : 'b') ?? "") + " astro-xxxxxx"}></div>`,
		},
		{
			name:   "empty",
			source: "<div class />",
			want:   `<div class="astro-xxxxxx"></div>`,
		},
		{
			name:   "template literal",
			source: "<div class=`${value}` />",
			want:   "<div class=`${value} astro-xxxxxx`></div>",
		},
		{
			name:   "component className not scoped",
			source: `<Component className="test" />`,
			want:   `<Component className="test astro-xxxxxx"></Component>`,
		},
		{
			name:   "component className expression",
			source: `<Component className={"test"} />`,
			want:   `<Component className={(("test") ?? "") + " astro-xxxxxx"}></Component>`,
		},
		{
			name:   "component className shorthand",
			source: "<Component {className} />",
			want:   `<Component className={className + " astro-xxxxxx"}></Component>`,
		},
		{
			name:   "element class:list",
			source: "<div class:list={{ a: true }} />",
			want:   `<div class:list={[({ a: true }), "astro-xxxxxx"]}></div>`,
		},
		{
			name:   "element class:list string",
			source: "<div class:list=\"weird but ok\" />",
			want:   `<div class:list="weird but ok astro-xxxxxx"></div>`,
		},
		{
			name:   "component class:list",
			source: "<Component class:list={{ a: true }} />",
			want:   `<Component class:list={[({ a: true }), "astro-xxxxxx"]}></Component>`,
		},
		{
			name:   "fault input currently accepted",
			source: `<A { 0>`,
			want:   `<A  0>={0>} class="astro-xxxxxx"></A>`,
		},
	}
}

func TestScopeHTML(t *testing.T) {
	tests := tests()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := handler.NewHandler(tt.source, "TestScopeHTML.astro")
			nodes, err := astro.ParseFragmentWithOptions(strings.NewReader(tt.source), &astro.Node{Type: astro.ElementNode, DataAtom: atom.Body, Data: atom.Body.String()}, astro.ParseOptionWithHandler(h))
			if err != nil {
				t.Error(err)
			}
			ScopeElement(nodes[0], TransformOptions{Scope: "xxxxxx"})
			var b strings.Builder
			astro.PrintToSource(&b, nodes[0])
			got := b.String()
			if tt.want != got {
				t.Errorf("\nFAIL: %s\n  want: %s\n  got:  %s", tt.name, tt.want, got)
			}
			// check whether another pass doesn't error
			nodes, err = astro.ParseFragmentWithOptions(strings.NewReader(tt.source), &astro.Node{Type: astro.ElementNode, DataAtom: atom.Body, Data: atom.Body.String()}, astro.ParseOptionWithHandler(h))
			if err != nil {
				t.Error(err)
			}

			ScopeElement(nodes[0], TransformOptions{Scope: "xxxxxx"})
			astro.PrintToSource(&b, nodes[0])
		})
	}
}

func FuzzScopeHTML(f *testing.F) {
	tests := tests()
	for _, tt := range tests {
		f.Add(tt.source) // Use f.Add to provide a seed corpus
	}
	f.Fuzz(func(t *testing.T, source string) {
		h := handler.NewHandler(source, "FuzzScopeHTML.astro")
		nodes, err := astro.ParseFragmentWithOptions(strings.NewReader(source), &astro.Node{Type: astro.ElementNode, DataAtom: atom.Body, Data: atom.Body.String()}, astro.ParseOptionWithHandler(h))
		if err != nil {
			t.Error(err)
		}
		// if the doc doesn't parse as an element node, we don't care
		if len(nodes) == 0 || nodes[0].Type != astro.ElementNode {
			t.Skip(nodes)
		}
		ScopeElement(nodes[0], TransformOptions{Scope: "xxxxxx"})
		// nodes[0] should still be an element node
		if len(nodes) == 0 || nodes[0].Type != astro.ElementNode {
			t.Errorf("`nodes[0]` is not an element node: %q\n nodes[0].Type: %q", source, nodes[0].Type)
		}
		var b strings.Builder
		astro.PrintToSource(&b, nodes[0])
		got := b.String()
		if !strings.Contains(got, "astro-xxxxxx") {
			t.Errorf("HTML scoping failed to include the astro scope\n source: %q\n got: %q\n `nodes[0].Data: %q", source, got, nodes[0].Data)
		}
		if utf8.ValidString(source) && !utf8.ValidString(got) {
			t.Errorf("HTML scoping produced invalid html string: %q", got)
		}
	})
}
