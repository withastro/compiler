package transform

import (
	"bytes"

	// "strings"

	tycho "github.com/snowpackjs/tycho/internal"
	"github.com/tdewolff/parse/css"
	a "golang.org/x/net/html/atom"
)

func ScopeStyle(styles []*tycho.Node, opts TransformOptions) {
	for _, n := range styles {
		if n.DataAtom == a.Style {
			n.Attr = append(n.Attr, tycho.Attribute{
				Key: "data-astro-id",
				Val: opts.Scope,
			})
			p := css.NewParser(bytes.NewBufferString(n.FirstChild.Data), false)
			out := ""
			for {
				gt, _, data := p.Next()
				if gt == css.ErrorGrammar {
					break
				} else if gt == css.AtRuleGrammar || gt == css.BeginAtRuleGrammar || gt == css.BeginRulesetGrammar || gt == css.DeclarationGrammar {
					out += string(data)
					if gt == css.DeclarationGrammar {
						out += ":"
					}
					for _, val := range p.Values() {
						if gt == css.BeginRulesetGrammar && val.TokenType == css.IdentToken {
							out += scopeRule(string(val.Data), opts) + " "
						} else {
							out += string(val.Data)
						}
					}
					if gt == css.BeginAtRuleGrammar || gt == css.BeginRulesetGrammar {
						out += "{"
					} else if gt == css.AtRuleGrammar || gt == css.DeclarationGrammar {
						out += ";"
					}
				} else {
					if gt == css.QualifiedRuleGrammar {
						for _, val := range p.Values() {
							out += scopeRule(string(val.Data), opts) + ", "
						}
					} else {
						out += string(data)
					}
				}
			}
			n.FirstChild.Data = out
		}
	}
}

func scopeRule(rule string, opts TransformOptions) string {
	scope := ".astro-" + opts.Scope
	return rule + scope
}
