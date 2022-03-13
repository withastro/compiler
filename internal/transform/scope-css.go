package transform

import (

	// "strings"

	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/lib/esbuild/css_parser"
	"github.com/withastro/compiler/lib/esbuild/css_printer"
	"github.com/withastro/compiler/lib/esbuild/logger"
	a "golang.org/x/net/html/atom"
)

// Take a slice of DOM nodes, and scope CSS within every <style> tag
func ScopeStyle(styles []*astro.Node, opts TransformOptions) bool {
	didScope := false
outer:
	for _, n := range styles {
		if n.DataAtom != a.Style {
			continue
		}
		if hasTruthyAttr(n, "global") {
			continue outer
		}
		didScope = true
		n.Attr = append(n.Attr, astro.Attribute{
			Key: "data-astro-id",
			Val: opts.Scope,
		})
		if n.FirstChild == nil {
			continue
		}
		tree := css_parser.Parse(logger.Log{}, logger.Source{Contents: n.FirstChild.Data}, css_parser.Options{MinifySyntax: false, MinifyWhitespace: false})
		result := css_printer.Print(tree, css_printer.Options{MinifyWhitespace: true, Scope: opts.Scope})
		n.FirstChild.Data = string(result.CSS)
		return didScope
	}

	return didScope
}

// Turn ".foo" into ".foo.astro-XXXXXX"
func scopeRule(id string, opts TransformOptions) string {
	return id + ".astro-" + opts.Scope
}

// Get list of elements that should be scoped
func globalElement(id string) bool {
	if NeverScopedElements[id] || NeverScopedSelectors[id] {
		return true
	}
	return false
}
