package transform

import (

	// "strings"

	"fmt"
	"strings"

	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/lib/esbuild/css_parser"
	"github.com/withastro/compiler/lib/esbuild/css_printer"
	"github.com/withastro/compiler/lib/esbuild/logger"
	a "golang.org/x/net/html/atom"
)

// Take a slice of DOM nodes, and scope CSS within every <style> tag
func ScopeStyle(styles []*astro.Node, opts TransformOptions) bool {
	didScope := false
	for _, n := range styles {
		if n.DataAtom != a.Style {
			continue
		}
		if hasTruthyAttr(n, "global") {
			fmt.Printf("Found `<style global>` in %s! Please migrate to the `is:global` directive.\n", opts.Filename)
			continue
		}
		if hasTruthyAttr(n, "is:global") {
			continue
		}
		if n.FirstChild == nil || strings.TrimSpace(n.FirstChild.Data) == "" {
			if !HasAttr(n, "define:vars") {
				continue
			}
		}
		didScope = true
		n.Attr = append(n.Attr, astro.Attribute{
			Key: "data-astro-id",
			Val: opts.Scope,
		})
		if n.FirstChild == nil || strings.TrimSpace(n.FirstChild.Data) == "" {
			continue
		}
		scopeStrategy := 1
		if opts.ScopedStyleStrategy == "class" {
			scopeStrategy = 2
		}

		// Use vendored version of esbuild internals to parse AST
		tree := css_parser.Parse(logger.Log{AddMsg: func(msg logger.Msg) {}}, logger.Source{Contents: n.FirstChild.Data}, css_parser.Options{MinifySyntax: false, MinifyWhitespace: true})
		// esbuild's internal `css_printer` has been modified to emit Astro scoped styles
		result := css_printer.Print(tree, css_printer.Options{MinifyWhitespace: true, Scope: opts.Scope, ScopeStrategy: scopeStrategy})
		n.FirstChild.Data = string(result.CSS)
	}

	return didScope
}

func GetDefineVars(styles []*astro.Node) []string {
	values := make([]string, 0)
	for _, n := range styles {
		if n.DataAtom != a.Style {
			continue
		}
		if !HasAttr(n, "define:vars") {
			continue
		}
		attr := GetAttr(n, "define:vars")
		if attr != nil {
			switch attr.Type {
			case astro.QuotedAttribute:
				values = append(values, fmt.Sprintf("'%s'", attr.Val))
			case astro.TemplateLiteralAttribute:
				values = append(values, fmt.Sprintf("`%s`", attr.Val))
			case astro.ExpressionAttribute:
				values = append(values, attr.Val)
			}
		}
	}

	return values
}
