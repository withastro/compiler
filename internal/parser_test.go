package astro

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/withastro/compiler/internal/handler"
	"github.com/withastro/compiler/internal/loc"
	"github.com/withastro/compiler/internal/test_utils"
	"golang.org/x/net/html/atom"
)

type ParserLocTest struct {
	name     string
	input    string
	expected []int
}

func TestParserLocation(t *testing.T) {
	Cases := []ParserLocTest{
		{
			"end tag I",
			`<div id="target"></div>`,
			[]int{1, 19},
		},
		{
			"end tag II",
			`<div class="TabBox">
	<div id="target" class="tab-bar">
		<div id="install-npm" class="active toggle"><h5>npm</h5></div>
		<div id="install-yarn" class="toggle"><h5>yarn</h5></div>
	</div>
</div>`,
			[]int{23, 184},
		},
		{
			"end tag III",
			`<span id="target" class:list={["link pixel variant", className]} {style}>
	<a {href}>
		<span><slot /></span>
	</a>
</span>
`,
			[]int{1, 118},
		},
		{
			"end tag VI",
			`<HeadingWrapper id="target">
		<h2 class="heading"><UIString key="rightSidebar.community" /></h2>
		{
			hideOnLargerScreens && (
				<svg
					class="chevron"
					xmlns="http://www.w3.org/2000/svg"
					viewBox="0 1 16 16"
					width="16"
					height="16"
					aria-hidden="true"
				>
					<path
						fill-rule="evenodd"
						d="M6.22 3.22a.75.75 0 011.06 0l4.25 4.25a.75.75 0 010 1.06l-4.25 4.25a.75.75 0 01-1.06-1.06L9.94 8 6.22 4.28a.75.75 0 010-1.06z"
					/>
				</svg>
			)
		}
	</HeadingWrapper>`,
			[]int{1, 492},
		},
	}

	runParserLocTest(t, Cases)
}

func runParserLocTest(t *testing.T, suite []ParserLocTest) {
	for _, tt := range suite {
		t.Run(tt.name, func(t *testing.T) {
			code := test_utils.Dedent(tt.input)

			doc, err := Parse(strings.NewReader(code))

			if err != nil {
				t.Error(err)
			}
			target := findTargetNode(doc)

			locs := make([]loc.Loc, 0)
			for _, start := range tt.expected {
				locs = append(locs, loc.Loc{Start: start})
			}

			if target == nil {
				t.Errorf("Loc = nil\nExpected = %v", locs)
				return
			}

			if !reflect.DeepEqual(target.Loc, locs) {
				t.Errorf("Loc = %v\nExpected = %v", target.Loc, locs)
			}
		})
	}
}

func walk(doc *Node, cb func(*Node)) {
	var f func(*Node)
	f = func(n *Node) {
		cb(n)
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
}

func findTargetNode(doc *Node) *Node {
	var target *Node = nil
	walk(doc, func(n *Node) {
		if target != nil {
			return
		}
		for _, attr := range n.Attr {
			if attr.Key == "id" && attr.Val == "target" {
				target = n
				return
			}
		}
	})
	return target
}

func fixturesParseFragmentWithOptions() []struct {
	name   string
	source string
	want   []*Node
} {
	return []struct {
		name   string
		source string
		want   []*Node
	}{
		{
			name:   "none",
			source: "<div />",
			want:   []*Node{{Type: ElementNode, DataAtom: atom.Div, Data: "div", Namespace: "", Loc: []loc.Loc{{Start: 1}}}},
		},
		{
			name:   "quoted",
			source: `<div class="test" />`,
			want:   nil,
		},
		{
			name:   "fault input currently accepted",
			source: `<A { 0>`,
			want:   []*Node{{Component: true, Type: ElementNode, DataAtom: 0, Data: "A", Namespace: "", Attr: []Attribute{{Namespace: "", Key: " 0>", KeyLoc: loc.Loc{Start: 4}, Val: "", ValLoc: loc.Loc{Start: 7}, Tokenizer: nil, Type: ShorthandAttribute}}, Loc: []loc.Loc{{Start: 1}}}},
		},
		{
			name:   "weird control characters",
			source: "\x00</F></a>",
			want:   nil,
		}}
}

func TestParseFragmentWithOptions(t *testing.T) {
	tests := fixturesParseFragmentWithOptions()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := handler.NewHandler(tt.source, "TestParseFragmentWithOptions.astro")
			nodes, err := ParseFragmentWithOptions(strings.NewReader(tt.source), &Node{Type: ElementNode, DataAtom: atom.Body, Data: atom.Body.String()}, ParseOptionWithHandler(h))
			if err != nil {
				t.Error(err)
			}
			assert.Equal(t, nodes, tt.want)
		})
	}

}

func FuzzParseFragmentWithOptions(f *testing.F) {
	tests := fixturesParseFragmentWithOptions()
	for _, tt := range tests {
		f.Add(tt.source) // Use f.Add to provide a seed corpus
	}
	f.Fuzz(func(t *testing.T, source string) {
		h := handler.NewHandler(source, "TestParseFragmentWithOptions.astro")
		_, err := ParseFragmentWithOptions(strings.NewReader(source), &Node{Type: ElementNode, DataAtom: atom.Body, Data: atom.Body.String()}, ParseOptionWithHandler(h))
		if err != nil {
			t.Error(err)
		}
	})
}
