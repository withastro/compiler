package transform

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	astro "github.com/snowpackjs/astro/internal"
	tycho "github.com/snowpackjs/astro/internal"
	"golang.org/x/net/html/atom"
)

func printToSource(buf bytes.Buffer, node *tycho.Node) string {
	if node.Type == tycho.ElementNode {
		buf.WriteString(fmt.Sprintf(`<%s`, node.Data))
		for _, attr := range node.Attr {
			if attr.Namespace != "" {
				buf.WriteString(attr.Namespace)
				buf.WriteString(":")
			}

			buf.WriteString(" ")
			switch attr.Type {
			case astro.QuotedAttribute:
				buf.WriteString(attr.Key)
				buf.WriteString("=")
				buf.WriteString(`"` + attr.Val + `"`)
			case astro.EmptyAttribute:
				buf.WriteString(attr.Key)
			case astro.ExpressionAttribute:
				buf.WriteString(attr.Key)
				buf.WriteString("=")
				buf.WriteString(`{` + strings.TrimSpace(attr.Val) + `}`)
			case astro.SpreadAttribute:
				buf.WriteString(`{...` + strings.TrimSpace(attr.Val) + `}`)
			case astro.ShorthandAttribute:
				buf.WriteString(attr.Key)
				buf.WriteString("=")
				buf.WriteString(`{` + strings.TrimSpace(attr.Key) + `}`)
			case astro.TemplateLiteralAttribute:
				buf.WriteString(attr.Key)
				buf.WriteString("=`" + strings.TrimSpace(attr.Val) + "`")
			}
		}
		buf.WriteString(fmt.Sprintf(`></%s>`, node.Data))
	}
	return buf.String()
}

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
			name:   "quoted trim",
			source: `<div class="test " />`,
			want:   `<div class="test astro-XXXXXX"></div>`,
		},
		{
			name:   "expression string",
			source: `<div class={"test"} />`,
			want:   `<div class={"test" + " astro-XXXXXX"}></div>`,
		},
		{
			name:   "expression function",
			source: `<div class={clsx({ [test]: true })} />`,
			want:   `<div class={clsx({ [test]: true }) + " astro-XXXXXX"}></div>`,
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
			name:   "component className",
			source: `<Component className="test" />`,
			want:   `<Component className="test astro-XXXXXX"></Component>`,
		},
		{
			name:   "component className expression",
			source: `<Component className={"test"} />`,
			want:   `<Component className={"test" + " astro-XXXXXX"}></Component>`,
		},
		{
			name:   "component className shorthand",
			source: "<Component {className} />",
			want:   `<Component className={className + " astro-XXXXXX"}></Component>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes, err := tycho.ParseFragment(strings.NewReader(tt.source), &tycho.Node{Type: astro.ElementNode, DataAtom: atom.Body, Data: atom.Body.String()})
			if err != nil {
				t.Error(err)
			}
			ScopeElement(nodes[0], TransformOptions{Scope: "XXXXXX"})
			got := printToSource(*bytes.NewBuffer([]byte{}), nodes[0])
			if tt.want != got {
				t.Error(fmt.Sprintf("\nFAIL: %s\n  want: %s\n  got:  %s", tt.name, tt.want, got))
			}
		})
	}
}
