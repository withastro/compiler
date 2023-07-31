package css_printer

import (
	"fmt"

	"github.com/withastro/compiler/lib/esbuild/css_ast"
	"github.com/withastro/compiler/lib/esbuild/css_lexer"
)

func (p *printer) printScopedSelector() bool {
	var str string
	if p.options.ScopeStrategy == ScopeStrategyWhere {
		str = fmt.Sprintf(":where(.astro-%s)", p.options.Scope)
	} else if p.options.ScopeStrategy == ScopeStrategyAttribute {
		str = fmt.Sprintf("[data-astro-hash-%s]", p.options.Scope)
	} else {
		str = fmt.Sprintf(".astro-%s", p.options.Scope)
	}
	p.print(str)
	return true
}

func (p *printer) printCompoundSelector(sel css_ast.CompoundSelector, isFirst bool, isLast bool) {
	scoped := false
	if !isFirst && sel.Combinator == "" {
		// A space is required in between compound selectors if there is no
		// combinator in the middle. It's fine to convert "a + b" into "a+b"
		// but not to convert "a b" into "ab".
		p.print(" ")
	}

	if sel.NestingSelector == css_ast.NestingSelectorPrefix {
		p.print("&")
		scoped = true
	}

	if sel.Combinator != "" {
		if !p.options.MinifyWhitespace {
			p.print(" ")
		}
		p.print(sel.Combinator)
		if !p.options.MinifyWhitespace {
			p.print(" ")
		}
	}

	if sel.TypeSelector != nil {
		whitespace := mayNeedWhitespaceAfter
		if len(sel.SubclassSelectors) > 0 {
			// There is no chance of whitespace before a subclass selector or pseudo
			// class selector
			whitespace = canDiscardWhitespaceAfter
		}
		if sel.TypeSelector.Name.Text == "*" {
			scoped = p.printScopedSelector()
		} else {
			p.printNamespacedName(*sel.TypeSelector, whitespace)
		}
		switch sel.TypeSelector.Name.Text {
		case "body", "html":
			scoped = true
		default:
			if !scoped {
				scoped = p.printScopedSelector()
			}
		}
	}

	for i, sub := range sel.SubclassSelectors {
		whitespace := mayNeedWhitespaceAfter

		// There is no chance of whitespace between subclass selectors
		if i+1 < len(sel.SubclassSelectors) {
			whitespace = canDiscardWhitespaceAfter
		}

		switch s := sub.(type) {
		case *css_ast.SSHash:
			p.print("#")

			// This deliberately does not use identHash. From the specification:
			// "In <id-selector>, the <hash-token>'s value must be an identifier."
			p.printIdent(s.Name, identNormal, whitespace)
			if !scoped {
				scoped = p.printScopedSelector()
			}

		case *css_ast.SSClass:
			p.print(".")
			p.printIdent(s.Name, identNormal, whitespace)
			if !scoped {
				scoped = p.printScopedSelector()
			}

		case *css_ast.SSAttribute:
			if !scoped {
				scoped = p.printScopedSelector()
			}
			p.print("[")
			p.printNamespacedName(s.NamespacedName, canDiscardWhitespaceAfter)
			if s.MatcherOp != "" {
				p.print(s.MatcherOp)
				printAsIdent := false

				// Print the value as an identifier if it's possible
				if css_lexer.WouldStartIdentifierWithoutEscapes(s.MatcherValue) {
					printAsIdent = true
					for _, c := range s.MatcherValue {
						if !css_lexer.IsNameContinue(c) {
							printAsIdent = false
							break
						}
					}
				}

				if printAsIdent {
					p.printIdent(s.MatcherValue, identNormal, canDiscardWhitespaceAfter)
				} else {
					p.printQuoted(s.MatcherValue)
				}
			}
			if s.MatcherModifier != 0 {
				p.print(" ")
				p.print(string(rune(s.MatcherModifier)))
			}
			p.print("]")

		case *css_ast.SSPseudoClass:
			p.printPseudoClassSelector(*s, whitespace)
			if s.Name == "global" || s.Name == "root" {
				scoped = true
			}
		}
	}

	if !scoped {
		p.printScopedSelector()
	}

	// It doesn't matter where the "&" goes since all non-prefix cases are
	// treated the same. This just always puts it as a suffix for simplicity.
	if sel.NestingSelector == css_ast.NestingSelectorPresentButNotPrefix {
		p.print("&")
	}
}

func (p *printer) printPseudoClassSelector(pseudo css_ast.SSPseudoClass, whitespace trailingWhitespace) {
	if pseudo.Name == "global" {
		if len(pseudo.Args) > 0 {
			p.printTokens(pseudo.Args, printTokensOpts{})
		} else {
			p.printIdent(pseudo.Name, identNormal, whitespace)
		}
	} else {
		if pseudo.IsElement {
			p.print("::")
		} else {
			p.print(":")
		}
		if len(pseudo.Args) > 0 {
			p.printIdent(pseudo.Name, identNormal, canDiscardWhitespaceAfter)
			p.print("(")
			p.printTokens(pseudo.Args, printTokensOpts{})
			p.print(")")
		} else {
			p.printIdent(pseudo.Name, identNormal, whitespace)
		}
	}
}
