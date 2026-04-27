package astro

import (
	"reflect"
	"strings"
	"testing"

	"github.com/withastro/compiler/internal/loc"
	"github.com/withastro/compiler/internal/test_utils"
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

type DuplicateAttributeTest struct {
	name           string
	input          string
	targetId       string
	expectedAttrs  map[string]string // key -> value
	expectedLength int               // expected number of attributes
}

func TestDuplicateAttributes(t *testing.T) {
	cases := []DuplicateAttributeTest{
		{
			name:     "duplicate class attributes - last wins",
			input:    `<div id="target" class="first" class="second" class="third"></div>`,
			targetId: "target",
			expectedAttrs: map[string]string{
				"id":    "target",
				"class": "third",
			},
			expectedLength: 2,
		},
		{
			name:     "duplicate data attributes - last wins",
			input:    `<div id="target" data-value="1" data-value="2"></div>`,
			targetId: "target",
			expectedAttrs: map[string]string{
				"id":         "target",
				"data-value": "2",
			},
			expectedLength: 2,
		},
		{
			name:     "multiple different duplicates",
			input:    `<div id="target" class="a" title="first" class="b" title="second"></div>`,
			targetId: "target",
			expectedAttrs: map[string]string{
				"id":    "target",
				"class": "b",
				"title": "second",
			},
			expectedLength: 3,
		},
		{
			name:     "no duplicates",
			input:    `<div id="target" class="test" title="hello"></div>`,
			targetId: "target",
			expectedAttrs: map[string]string{
				"id":    "target",
				"class": "test",
				"title": "hello",
			},
			expectedLength: 3,
		},
		{
			name:     "duplicate with namespace - last wins",
			input:    `<svg id="target" xmlns:xlink="http://first" xmlns:xlink="http://second"></svg>`,
			targetId: "target",
			expectedAttrs: map[string]string{
				"id": "target",
			},
			expectedLength: 2, // id + xlink namespace
		},
		{
			name:     "three duplicates of same attribute",
			input:    `<div id="target" data-test="one" data-test="two" data-test="three"></div>`,
			targetId: "target",
			expectedAttrs: map[string]string{
				"id":        "target",
				"data-test": "three",
			},
			expectedLength: 2,
		},
		{
			name:     "duplicate aria attributes",
			input:    `<button id="target" aria-label="first" aria-label="second"></button>`,
			targetId: "target",
			expectedAttrs: map[string]string{
				"id":         "target",
				"aria-label": "second",
			},
			expectedLength: 2,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			code := test_utils.Dedent(tt.input)
			doc, err := Parse(strings.NewReader(code))

			if err != nil {
				t.Errorf("Parse error: %v", err)
				return
			}

			target := findTargetNode(doc)
			if target == nil {
				t.Error("Target node not found")
				return
			}

			if len(target.Attr) != tt.expectedLength {
				t.Errorf("Expected %d attributes, got %d", tt.expectedLength, len(target.Attr))
			}

			for key, expectedVal := range tt.expectedAttrs {
				found := false
				for _, attr := range target.Attr {
					attrKey := attr.Key
					if attr.Namespace != "" {
						attrKey = attr.Namespace + ":" + attr.Key
					}
					if attrKey == key {
						found = true
						if attr.Val != expectedVal {
							t.Errorf("For attribute %s: expected value %q, got %q", key, expectedVal, attr.Val)
						}
						break
					}
				}
				if !found {
					t.Errorf("Expected attribute %s not found", key)
				}
			}
		})
	}
}
