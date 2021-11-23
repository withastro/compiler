package transform

import (
	"fmt"
	"strings"
	"testing"

	astro "github.com/snowpackjs/astro/internal"
)

func TestTransformScoping(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "basic",
			source: `
				<style>div { color: red }</style>
				<div />
			`,
			want: `<div class="astro-XXXXXX"></div>`,
		},
		{
			name: "global empty",
			source: `
				<style global>div { color: red }</style>
				<div />
			`,
			want: `<div></div>`,
		},
		{
			name: "global true",
			source: `
				<style global={true}>div { color: red }</style>
				<div />
			`,
			want: `<div></div>`,
		},
		{
			name: "global string",
			source: `
				<style global="">div { color: red }</style>
				<div />
			`,
			want: `<div></div>`,
		},
		{
			name: "global string true",
			source: `
				<style global="true">div { color: red }</style>
				<div />
			`,
			want: `<div></div>`,
		},
		{
			name: "scoped multiple",
			source: `
				<style>div { color: red }</style>
				<style>div { color: green }</style>
				<div />
			`,
			want: `<div class="astro-XXXXXX"></div>`,
		},
		{
			name: "global multiple",
			source: `
				<style global>div { color: red }</style>
				<style global>div { color: green }</style>
				<div />
			`,
			want: `<div></div>`,
		},
		{
			name: "mixed multiple",
			source: `
				<style>div { color: red }</style>
				<style global>div { color: green }</style>
				<div />
			`,
			want: `<div class="astro-XXXXXX"></div>`,
		},
		{
			name: "multiple scoped :global",
			source: `
				<style>:global(test-2) {}</style>
				<style>:global(test-1) {}</style>
				<div />
			`,
			want: `<div class="astro-XXXXXX"></div>`,
		},
	}
	var b strings.Builder
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b.Reset()
			doc, err := astro.Parse(strings.NewReader(tt.source))
			if err != nil {
				t.Error(err)
			}
			ExtractStyles(doc)
			Transform(doc, TransformOptions{Scope: "XXXXXX"})
			astro.PrintToSource(&b, doc.LastChild.FirstChild.NextSibling.FirstChild)
			got := b.String()
			if tt.want != got {
				t.Error(fmt.Sprintf("\nFAIL: %s\n  want: %s\n  got:  %s", tt.name, tt.want, got))
			}
		})
	}
}

func TestFullTransform(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "top-level component with leading style",
			source: `<style>:root{}</style><Component><h1>Hello world</h1></Component>
			`,
			want: `<Component><h1>Hello world</h1></Component>`,
		},
		{
			name: "top-level component with leading style body",
			source: `<style>:root{}</style><Component><div><h1>Hello world</h1></div></Component>
			`,
			want: `<Component><div><h1>Hello world</h1></div></Component>`,
		},
		{
			name: "top-level component with trailing style",
			source: `<Component><h1>Hello world</h1></Component><style>:root{}</style>
			`,
			want: `<Component><h1>Hello world</h1></Component>`,
		},
		{
			name:   "respects explicitly authored elements",
			source: `<html><Component /></html>`,
			want:   `<html><Component></Component></html>`,
		},
		{
			name:   "respects explicitly authored elements 2",
			source: `<head></head><Component />`,
			want:   `<html><head></head><Component></Component></html>`,
		},
		{
			name:   "respects explicitly authored elements 3",
			source: `<body><Component /></body>`,
			want:   `<html><head></head><body><Component></Component></body></html>`,
		},
		{
			name:   "removes implicitly generated elements",
			source: `<Component />`,
			want:   `<Component></Component>`,
		},
		{
			name:   "works with nested components",
			source: `<style></style><A><div><B /></div></A>`,
			want:   `<A><div><B></B></div></A>`,
		},
		{
			name: "does not remove trailing siblings",
			source: `<title>Title</title>
<span />
<Component />
<span />`,
			want: `<html><head><title>Title</title>
</head><body><span></span>
<Component></Component>
<span></span></body></html>`,
		},
	}
	var b strings.Builder
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b.Reset()
			doc, err := astro.Parse(strings.NewReader(tt.source))
			if err != nil {
				t.Error(err)
			}
			ExtractStyles(doc)
			// Clear doc.Styles to avoid scoping behavior, we're not testing that here
			doc.Styles = make([]*astro.Node, 0)
			Transform(doc, TransformOptions{})
			astro.PrintToSource(&b, doc)
			got := strings.TrimSpace(b.String())
			if tt.want != got {
				t.Error(fmt.Sprintf("\nFAIL: %s\n  want: %s\n  got:  %s", tt.name, tt.want, got))
			}
		})
	}
}
