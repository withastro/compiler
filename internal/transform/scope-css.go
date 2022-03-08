package transform

import (
	"bytes"
	"strings"

	// "strings"

	astro "astro.build/x/compiler/internal"
	"github.com/tdewolff/parse/css"
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
		p := css.NewParser(bytes.NewBufferString(n.FirstChild.Data), false)
		out := ""

		isKeyframes := false    // if we’re inside @keyframes, there’s nothing to scope
		keyframeCurlyCount := 0 // keep track of open "{"s inside @keyframes
		declaration := ""

	walk:
		for {
			gt, _, data := p.Next()

			switch gt {
			case css.ErrorGrammar:
				if len(string(data)) > 0 {
					out += string(data) // this will happen for invalid or unexpected CSS. Try and retain output as much as possible without throwing
				} else {
					break walk // an unrecoverable error occurred, or the end has been reached
				}
			case css.CommentGrammar:
				out += string(data)
			case css.EndAtRuleGrammar,
				css.EndRulesetGrammar:
				out += "}"
			case
				css.BeginAtRuleGrammar,
				css.BeginRulesetGrammar,
				css.DeclarationGrammar,
				css.QualifiedRuleGrammar:

				// prelude
				switch gt {
				case css.AtRuleGrammar,
					css.BeginAtRuleGrammar:
					out += string(data)
					if string(data) == "@keyframes" {
						isKeyframes = true
						keyframeCurlyCount = 0
					}
				case css.DeclarationGrammar:
					out += string(data) + ":"
					declaration = string(data)
				default:
				}

				// main selector
				parenCount := 0          // keeps track of open parens. scoping can’t happen inside parens (:not(), :where(), etc.)
				isBracket := false       // keeps track of attr brackets (can’t nest like parens, so it’s simply ”open”/“close”)
				isGlobal := false        // keeps track of :global() function (no scope, and omit from output)
				isElement := true        // keeps track of base element selectors (e.g. body, h1). Elements must be assumed ("true") until ".", "#", etc. are encountered
				isGlobalElement := false // keeps track of <body>, <html>, and other protected elements (isElement will always be true as well)
				isPseudoState := false   // keeps track of pseudo state/element context (i.e. ensures :hover or ::before don’t get scoped). This is "false" until ":" is encountered
				nextValues := p.Values()
				for n, val := range nextValues {
					strVal := string(val.Data)

					// if inside @keyframes, don’t transform what’s there
					if isKeyframes {
						if strVal == "{" {
							keyframeCurlyCount++
						} else if strVal == "}" {
							keyframeCurlyCount--
						}

						// Inside of this case, we only want to break out when keyframeCurlyCount is -1
						// since 0 is the default
						if keyframeCurlyCount < 0 {
							isKeyframes = false
						}
						out += strVal
						continue
					}

					switch strVal {
					case ".",
						"#":
						isPseudoState = false
						isElement = false
						out += strVal
					case ":":
						isPseudoState = true
						// look ahead to see if this is the start of ":global(".
						// If so, omit from output and start global state
						if len(nextValues) > n+1 && string(nextValues[n+1].Data) == "global(" {
							isGlobal = true
						} else {
							// if not the start of ":global(", then include in output
							out += strVal
						}
					case "global(":
						parenCount++
						// omit from output
					case "(":
						parenCount++
						out += strVal
						isElement = true
						isPseudoState = false
					case ")":
						parenCount--
						if !isGlobal || parenCount != 0 {
							out += strVal // output only if this doesn’t close ":global("
						}
					case "[":
						isBracket = true
						isElement = false
						isPseudoState = false

						// if there is no selector before an attribute selector and we're not in a delcaration, assume "*"
						if n == 0 && declaration == "" {
							out += scopeRule("", opts)
						}
						out += strVal
					case "]":
						isBracket = false
						out += strVal
					case "{":
						if isKeyframes {
							keyframeCurlyCount++
						}
						isElement = true
						isPseudoState = false
						out += strVal
					case "}":
						if isKeyframes {
							keyframeCurlyCount--
						}
						if keyframeCurlyCount == 0 {
							isKeyframes = false
						}
						out += strVal
					case "*":
						if parenCount == 0 && !isGlobal {
							out += scopeRule("", opts) // turns "*" into ".astro-XXXXXX" rather than "*.astro-XXXXXX"
						} else {
							out += strVal
						}
					default:
						// handle IDs with parens attached
						if strings.Contains(strVal, "(") {
							parenCount++ // if new paren opened, count it
							isElement = true
							isPseudoState = false
						}

						// if this is an element, check if it’s <body>, etc.
						if isElement && globalElement(strVal) {
							isGlobalElement = true
						}

						// whitespace tokens are used to reset chained classes and functions
						if val.TokenType == css.WhitespaceToken {
							// important: global elements like <body> may have classes that should not be scoped
							if isElement && isGlobalElement {
								isGlobalElement = false
							}

							// important: :global() might be chained (:global().some-class)
							// so keep it active until whitespace is reached after final paren
							if isGlobal && parenCount == 0 {
								isGlobal = false
							}
						}

						// scope class
						isCssSelector := (gt == css.BeginRulesetGrammar || gt == css.QualifiedRuleGrammar) && (val.TokenType == css.IdentToken || val.TokenType == css.HashToken)
						if isCssSelector && // don’t scope @media and other non-class specifiers
							!isPseudoState && // don’t scope pseudostates
							!isGlobal && // don’t scope in :global() scope
							!isGlobalElement &&
							!isBracket && // don’t scope within element brackets
							parenCount == 0 { // don’t scope within parens like :not()
							out += scopeRule(strVal, opts)
						} else {
							// otherwise, append output
							out += strVal
						}

						// reset state
						isElement = true
						isPseudoState = false
						declaration = ""
					}
				}

				// generate tail of next rule
				switch gt {
				case css.BeginAtRuleGrammar,
					css.BeginRulesetGrammar:
					out += "{"
				case css.DeclarationGrammar,
					css.EndRulesetGrammar,
					css.EndAtRuleGrammar:
					out += ";"
				case css.QualifiedRuleGrammar:
					out += ","
				}
			default:
				strData := string(data)
				out += strData
				for _, val := range p.Values() {
					strVal := string(val.Data)
					// handle CSS variables
					if strings.HasPrefix(strData, "--") {
						out += ":"
					}
					out += strVal
				}
				out += ";"
			}
		}
		n.FirstChild.Data = out
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
