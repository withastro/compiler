package js_scanner

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
	"github.com/withastro/compiler/internal/loc"
)

// FindTopLevelReturns scans JavaScript/TypeScript source code and returns the
// byte positions of all `return` statements that are at the top level (i.e., not
// inside any function, arrow function, method, or class method).
//
// This is used to transform top-level returns into throws in TSX output, because
// top-level returns are valid in Astro frontmatter but cause TypeScript parsing errors.
func FindTopLevelReturns(source []byte) []int {
	l := js.NewLexer(parse.NewInputBytes(source))
	i := 0
	returns := make([]int, 0)

	// We need to track "function scope depth" - returns are only top-level
	// if they're not inside any function body.
	//
	// The challenge is distinguishing between:
	// - `if (cond) { return; }` - top-level return (inside if block)
	// - `function f() { return; }` - not top-level (inside function)
	// - `() => { return; }` - not top-level (inside arrow function)
	// - `class C { method() { return; } }` - not top-level (inside class method)
	// - `{ method() { return; } }` - not top-level (inside object method)
	// - `{ ['computed']() { return; } }` - not top-level (computed method)
	//
	// Strategy: Track when we're expecting a function body to start.
	// A function body starts with `{` after:
	// - `function` keyword followed by optional name and `()`
	// - `=>` (arrow function)
	// - `identifier()` where `{` follows (method shorthand in objects/classes)
	// - `[expr]()` where `{` follows (computed method in objects/classes)
	functionScopeStack := make([]int, 0) // stack of brace depths when entering function scopes
	braceDepth := 0
	bracketDepth := 0

	// Track parentheses depth to detect when we close params
	parenDepth := 0
	parenDepthAtFunctionStart := -1 // the paren depth when we saw `function` keyword

	// Track if we're expecting a function body
	expectingFunctionBody := false

	// Track method shorthand: identifier + () + { = method shorthand
	// We need to track the paren depth when we see an identifier, so we know
	// if the identifier is BEFORE the parens (method shorthand) or INSIDE them (not method)
	// E.g., `method() { }` vs `if (condition) { }`
	identParenDepth := -1 // paren depth when we last saw an identifier at current level

	// Track that we actually went through parens after seeing identifier
	// This distinguishes `method() {` from `class Foo {`
	wentThroughParensForMethod := false

	// Track computed property: [expr] + () + { = computed method
	// After we see `]` that closes a bracket at the same level, we may have a computed method
	sawCloseBracketForMethod := false

	for {
		token, value := l.Next()

		// Handle regex vs division ambiguity
		if token == js.DivToken || token == js.DivEqToken {
			if i+1 < len(source) {
				lns := bytes.Split(source[i+1:], []byte{'\n'})
				if bytes.Contains(lns[0], []byte{'/'}) {
					token, value = l.RegExp()
				}
			}
		}

		if token == js.ErrorToken {
			if l.Err() != io.EOF {
				return returns
			}
			break
		}

		// Skip whitespace and comments
		if token == js.WhitespaceToken || token == js.LineTerminatorToken ||
			token == js.CommentToken || token == js.CommentLineTerminatorToken {
			i += len(value)
			continue
		}

		// Track identifiers (for method shorthand pattern: identifier + () + {)
		// Only track if we're not already inside parens from something else
		if js.IsIdentifier(token) {
			identParenDepth = parenDepth
			wentThroughParensForMethod = false
			sawCloseBracketForMethod = false
			i += len(value)
			continue
		}

		// Track parentheses
		if js.IsPunctuator(token) {
			if value[0] == '(' {
				parenDepth++
				i += len(value)
				continue
			} else if value[0] == ')' {
				parenDepth--
				// If we close parens back to function start level, we expect function body next
				if parenDepthAtFunctionStart >= 0 && parenDepth == parenDepthAtFunctionStart {
					expectingFunctionBody = true
					parenDepthAtFunctionStart = -1
				}
				// Check if we just closed parens back to where we saw an identifier
				// This means we went through `identifier()` pattern
				if identParenDepth >= 0 && parenDepth == identParenDepth {
					wentThroughParensForMethod = true
				}
				i += len(value)
				continue
			}
		}

		// Track square brackets for computed properties [expr]
		if js.IsPunctuator(token) {
			if value[0] == '[' {
				bracketDepth++
				sawCloseBracketForMethod = false
				i += len(value)
				continue
			} else if value[0] == ']' {
				bracketDepth--
				// Mark that we just closed a bracket - this could be a computed property name
				// The next thing should be `()` for it to be a method
				sawCloseBracketForMethod = true
				identParenDepth = -1
				i += len(value)
				continue
			}
		}

		// Detect arrow function: `=>` means we expect a function body
		if token == js.ArrowToken {
			expectingFunctionBody = true
			identParenDepth = -1
			sawCloseBracketForMethod = false
			i += len(value)
			continue
		}

		// Track function keywords - after `function`, we wait for `(` then `)`
		if token == js.FunctionToken {
			parenDepthAtFunctionStart = parenDepth
			identParenDepth = -1
			sawCloseBracketForMethod = false
			i += len(value)
			continue
		}

		// Track braces
		if js.IsPunctuator(token) {
			if value[0] == '{' {
				// Check if this brace opens a function body
				// This happens after:
				// 1. `function name()` or `function()`
				// 2. `=>`
				// 3. `identifier()` (method shorthand) - identifier followed by () then {
				// 4. `[expr]()` (computed method) - sawCloseBracketForMethod was set and we went through ()
				isMethodShorthand := wentThroughParensForMethod
				isComputedMethod := sawCloseBracketForMethod
				if expectingFunctionBody || isMethodShorthand || isComputedMethod {
					// Entering a function scope
					functionScopeStack = append(functionScopeStack, braceDepth)
					expectingFunctionBody = false
				}
				identParenDepth = -1
				wentThroughParensForMethod = false
				sawCloseBracketForMethod = false
				braceDepth++
				i += len(value)
				continue
			} else if value[0] == '}' {
				braceDepth--
				// Check if we're exiting a function scope
				if len(functionScopeStack) > 0 && braceDepth == functionScopeStack[len(functionScopeStack)-1] {
					functionScopeStack = functionScopeStack[:len(functionScopeStack)-1]
				}
				identParenDepth = -1
				wentThroughParensForMethod = false
				sawCloseBracketForMethod = false
				i += len(value)
				continue
			}
		}

		// Reset identifier tracking on other tokens (but preserve sawCloseBracketForMethod
		// through parens so `[expr]()` works)
		identParenDepth = -1

		// A return is top-level if we're not inside any function scope
		if token == js.ReturnToken && len(functionScopeStack) == 0 {
			returns = append(returns, i)
		}

		i += len(value)
	}

	return returns
}

type HoistedScripts struct {
	Hoisted     [][]byte
	HoistedLocs []loc.Loc
	Body        [][]byte
	BodyLocs    []loc.Loc
}

func HoistExports(source []byte) HoistedScripts {
	shouldHoist := bytes.Contains(source, []byte("export"))
	if !shouldHoist {
		body := make([][]byte, 0)
		body = append(body, source)
		bodyLocs := make([]loc.Loc, 0)
		bodyLocs = append(bodyLocs, loc.Loc{Start: 0})
		return HoistedScripts{
			Body:     body,
			BodyLocs: bodyLocs,
		}
	}

	l := js.NewLexer(parse.NewInputBytes(source))
	i := 0
	end := 0

	hoisted := make([][]byte, 0)
	hoistedLocs := make([]loc.Loc, 0)
	body := make([][]byte, 0)
	bodyLocs := make([]loc.Loc, 0)
	pairs := make(map[byte]int)

	// Let's lex the script until we find what we need!
outer:
	for {
		token, value := l.Next()

		if token == js.DivToken || token == js.DivEqToken {
			lns := bytes.Split(source[i+1:], []byte{'\n'})
			if bytes.Contains(lns[0], []byte{'/'}) {
				token, value = l.RegExp()
			}
		}

		if token == js.ErrorToken {
			if l.Err() != io.EOF {
				body := make([][]byte, 0)
				body = append(body, source)
				bodyLocs := make([]loc.Loc, 0)
				bodyLocs = append(bodyLocs, loc.Loc{Start: 0})
				return HoistedScripts{
					Hoisted:     hoisted,
					HoistedLocs: hoistedLocs,
					Body:        body,
					BodyLocs:    bodyLocs,
				}
			}
			break
		}

		// Common delimiters. Track their length, then skip.
		if token == js.WhitespaceToken || token == js.LineTerminatorToken || token == js.SemicolonToken {
			i += len(value)
			continue
		}

		// Exports should be consumed until all opening braces are closed,
		// a specifier is found, and a line terminator has been found
		if token == js.ExportToken {
			flags := make(map[string]bool)
			tokensFound := make(map[string]bool)
			foundIdent := false
			foundSemicolonOrLineTerminator := false
			foundBody := false
			start := i
			i += len(value)
			for {
				next, nextValue := l.Next()
				if next == js.DivToken || next == js.DivEqToken {
					lns := bytes.Split(source[i+1:], []byte{'\n'})
					if bytes.Contains(lns[0], []byte{'/'}) {
						next, nextValue = l.RegExp()
					}
				}
				i += len(nextValue)
				flags[string(nextValue)] = true
				tokensFound[string(nextValue)] = true

				if next == js.ErrorToken && l.Err() == io.EOF {
					foundSemicolonOrLineTerminator = true
				}

				if js.IsIdentifier(next) {
					if isKeyword(nextValue) && next != js.FromToken {
						continue
					}
					if string(nextValue) == "type" {
						continue
					}
					if !foundIdent {
						foundIdent = true
					}
				} else if next == js.LineTerminatorToken || next == js.SemicolonToken {
					if next == js.LineTerminatorToken && i < len(source) && (source[i] == '&' || source[i] == '|') {
						continue
					}
					if (flags["function"] || flags["=>"] || flags["interface"]) && !foundBody {
						continue
					}
					if flags["&"] || flags["="] {
						continue
					}
					if pairs['('] > 0 {
						continue
					}

					foundSemicolonOrLineTerminator = true
				} else if js.IsPunctuator(next) {
					if nextValue[0] == '{' {
						if flags["function"] {
							// Curly braces can occur in a function parameter destructuring, which we don't want to consider
							foundBody = foundBody || pairs['('] == 0
						} else if flags["=>"] {
							// Arrow can also occur in type definition before arrow function body (which we don't want to consider), but `=` cannot
							foundBody = foundBody || tokensFound["="]
						} else {
							foundBody = true
						}
					}
					if nextValue[0] == '{' || nextValue[0] == '(' || nextValue[0] == '[' {
						flags[string(nextValue[0])] = true
						pairs[nextValue[0]]++
					} else if nextValue[0] == '}' {
						pairs['{']--
					} else if nextValue[0] == ')' {
						pairs['(']--
					} else if nextValue[0] == ']' {
						pairs['[']--
					}
				} else {
					// Sometimes, exports are written in multiple lines, like
					//
					// export const foo =
					//   [...]
					// export type Props = ThisProps &
					// 	 SomeWeirdType<{ thatsSuperLong: SoItEndsUpFormattedLikeThis }>
					//
					// So, we omit the semicolon check if the line ends up with one of these
					if flags["&"] && nextValue[0] != '&' {
						flags["&"] = false
					}
					if flags["="] && nextValue[0] != '=' {
						flags["="] = false
					}
				}

				if foundIdent && foundSemicolonOrLineTerminator && pairs['{'] == 0 && pairs['('] == 0 && pairs['['] == 0 {
					hoisted = append(hoisted, source[start:i])
					hoistedLocs = append(hoistedLocs, loc.Loc{Start: start})
					if end < start {
						body = append(body, source[end:start])
						bodyLocs = append(bodyLocs, loc.Loc{Start: end})
					}
					end = i
					continue outer
				}

				if next == js.ErrorToken {
					if l.Err() != io.EOF {
						body := make([][]byte, 0)
						body = append(body, source)
						bodyLocs := make([]loc.Loc, 0)
						bodyLocs = append(bodyLocs, loc.Loc{Start: 0})
						return HoistedScripts{
							Hoisted:     hoisted,
							HoistedLocs: hoistedLocs,
							Body:        body,
							BodyLocs:    bodyLocs,
						}
					}
					break outer
				}
			}
		}

		// Track opening and closing braces
		if js.IsPunctuator(token) {
			if value[0] == '{' || value[0] == '(' || value[0] == '[' {
				pairs[value[0]]++
				i += len(value)
				continue
			} else if value[0] == '}' {
				pairs['{']--
			} else if value[0] == ')' {
				pairs['(']--
			} else if value[0] == ']' {
				pairs['[']--
			}
		}

		// Track our current position
		i += len(value)
	}

	body = append(body, source[end:])
	bodyLocs = append(bodyLocs, loc.Loc{Start: end})

	return HoistedScripts{
		Hoisted:     hoisted,
		HoistedLocs: hoistedLocs,
		Body:        body,
		BodyLocs:    bodyLocs,
	}
}

func isKeyword(value []byte) bool {
	return js.Keywords[string(value)] != 0
}

// isPropsAliasing checks if we're in a Props aliasing context (import { Props as X })
// rather than destructuring with 'as' property ({ as: Component })
func isPropsAliasing(idents []string) bool {
	return len(idents) > 0 && idents[len(idents)-1] == "Props"
}

func HoistImports(source []byte) HoistedScripts {
	imports := make([][]byte, 0)
	importLocs := make([]loc.Loc, 0)
	body := make([][]byte, 0)
	bodyLocs := make([]loc.Loc, 0)
	prev := 0
	for i, statement := NextImportStatement(source, 0); i > -1 && i < len(source)+1; i, statement = NextImportStatement(source, i) {
		bodyLocs = append(bodyLocs, loc.Loc{Start: prev})
		body = append(body, source[prev:statement.Span.Start])
		imports = append(imports, statement.Value)
		importLocs = append(importLocs, loc.Loc{Start: statement.Span.Start})
		prev = i
	}
	if prev == 0 {
		bodyLocs = append(bodyLocs, loc.Loc{Start: 0})
		body = append(body, source)
		return HoistedScripts{Body: body, BodyLocs: bodyLocs}
	}
	bodyLocs = append(bodyLocs, loc.Loc{Start: prev})
	body = append(body, source[prev:])
	return HoistedScripts{Hoisted: imports, HoistedLocs: importLocs, Body: body, BodyLocs: bodyLocs}
}

func HasGetStaticPaths(source []byte) bool {
	ident := []byte("getStaticPaths")
	if !bytes.Contains(source, ident) {
		return false
	}

	exports := HoistExports(source)
	for _, statement := range exports.Hoisted {
		if bytes.Contains(statement, ident) {
			return true
		}
	}
	return false
}

type Props struct {
	Ident     string
	Statement string
	Generics  string
}

func GetPropsType(source []byte) Props {
	defaultPropType := "Record<string, any>"
	ident := defaultPropType
	genericsIdents := make([]string, 0)
	generics := ""
	statement := ""

	if !bytes.Contains(source, []byte("Props")) {
		return Props{
			Ident:     ident,
			Statement: statement,
			Generics:  generics,
		}
	}
	l := js.NewLexer(parse.NewInputBytes(source))
	i := 0
	pairs := make(map[byte]int)
	idents := make([]string, 0)

	start := 0
	end := 0

outer:
	for {
		token, value := l.Next()

		if token == js.DivToken || token == js.DivEqToken {
			if len(source) > i {
				lns := bytes.Split(source[i+1:], []byte{'\n'})
				if bytes.Contains(lns[0], []byte{'/'}) {
					token, value = l.RegExp()
				}
			}
		}

		if token == js.ErrorToken {
			if l.Err() != io.EOF {
				return Props{
					Ident: ident,
				}
			}
			break
		}

		// Common delimiters. Track their length, then skip.
		if token == js.WhitespaceToken || token == js.LineTerminatorToken || token == js.SemicolonToken {
			i += len(value)
			continue
		}

		if token == js.ExtendsToken {
			if bytes.Equal(value, []byte("extends")) {
				idents = append(idents, "extends")
			}
			i += len(value)
			continue
		}

		if pairs['{'] == 0 && pairs['('] == 0 && pairs['['] == 0 && pairs['<'] == 1 && token == js.CommaToken {
			idents = make([]string, 0)
			i += len(value)
			continue
		}

		if js.IsIdentifier(token) {
			if isKeyword(value) {
				// fix(#814): fix Props detection when using `{ Props as SomethingElse }`
				// fix(#927): only reset Props when 'as' follows 'Props' in the same context
				if ident == "Props" && string(value) == "as" && isPropsAliasing(idents) {
					start = 0
					ident = defaultPropType
					idents = make([]string, 0)
				}
				i += len(value)
				continue
			}
			if pairs['<'] == 1 && pairs['{'] == 0 {
				foundExtends := false
				for _, id := range idents {
					if id == "extends" {
						foundExtends = true
					}
				}
				if !foundExtends {
					genericsIdents = append(genericsIdents, string(value))
				}
				i += len(value)
				continue
			}
			// Note: do not check that `pairs['{'] == 0` to support named imports
			if pairs['('] == 0 && pairs['['] == 0 && string(value) == "Props" {
				ident = "Props"
			}
			idents = append(idents, string(value))
			i += len(value)
			continue
		}

		if bytes.ContainsAny(value, "<>") {
			if len(idents) > 0 && idents[len(idents)-1] == "Props" {
				start = i
				ident = "Props"
				idents = make([]string, 0)
			}
			for _, c := range value {
				if c == '<' {
					pairs['<']++
					i += len(value)
					continue
				}
				if c == '>' {
					pairs['<']--
					if pairs['<'] == 0 {
						end = i
						// Important: only break out if we've already found `Props`!
						if ident != defaultPropType {
							break outer
						} else {
							continue
						}
					}
				}
			}
		}

		if token == js.QuestionToken || (pairs['{'] == 0 && token == js.ColonToken) {
			idents = make([]string, 0)
			idents = append(idents, "extends")
		}

		// Track opening and closing braces
		if js.IsPunctuator(token) {
			if value[0] == '{' || value[0] == '(' || value[0] == '[' {
				idents = make([]string, 0)
				pairs[value[0]]++
				i += len(value)
				continue
			} else if value[0] == '}' {
				pairs['{']--
				if pairs['<'] == 0 && pairs['{'] == 0 && ident != defaultPropType {
					end = i
					break outer
				}
			} else if value[0] == ')' {
				pairs['(']--
			} else if value[0] == ']' {
				pairs['[']--
			}
		}

		// Track our current position
		i += len(value)
	}
	if start > 0 && len(genericsIdents) > 0 && ident != defaultPropType {
		generics = fmt.Sprintf("<%s>", strings.Join(genericsIdents, ", "))
		statement = strings.TrimSpace(string(source[start:end]))
	}

	return Props{
		Ident:     ident,
		Statement: statement,
		Generics:  generics,
	}
}

func IsIdentifier(value []byte) bool {
	valid := true
	for i, b := range value {
		if i == 0 {
			valid = js.IsIdentifierStart([]byte{b})
		} else if i < len(value)-1 {
			valid = js.IsIdentifierContinue([]byte{b})
		} else {
			valid = js.IsIdentifierEnd([]byte{b})
		}
		if !valid {
			break
		}
	}
	return valid
}

func GetObjectKeys(source []byte) [][]byte {
	keys := make([][]byte, 0)
	pairs := make(map[byte]int)

	if source[0] == '{' && source[len(source)-1] == '}' {
		l := js.NewLexer(parse.NewInputBytes(source[1 : len(source)-1]))
		i := 0
		var prev js.TokenType

		for {
			token, value := l.Next()
			openPairs := pairs['{'] > 0 || pairs['('] > 0 || pairs['['] > 0

			if token == js.DivToken || token == js.DivEqToken {
				lns := bytes.Split(source[i+1:], []byte{'\n'})
				if bytes.Contains(lns[0], []byte{'/'}) {
					token, value = l.RegExp()
				}
			}
			i += len(value)

			if token == js.ErrorToken {
				return keys
			}

			if js.IsPunctuator(token) {
				if value[0] == '{' || value[0] == '(' || value[0] == '[' {
					pairs[value[0]]++
					continue
				} else if value[0] == '}' {
					pairs['{']--
				} else if value[0] == ')' {
					pairs['(']--
				} else if value[0] == ']' {
					pairs['[']--
				}
			}

			if prev != js.ColonToken {
				push := func() {
					if token != js.StringToken {
						keys = append(keys, value)
					} else {
						key := value[1 : len(value)-1]
						ident := string(key)
						if !IsIdentifier(key) {
							ident = strcase.ToLowerCamel(string(key))
						}
						if string(key) == ident {
							keys = append(keys, []byte(key))
						} else {
							keys = append(keys, []byte(fmt.Sprintf("%s: %s", value, ident)))
						}
					}
				}
				if !openPairs && (token == js.IdentifierToken || token == js.StringToken) {
					push()
				} else if pairs['['] == 1 && token == js.StringToken {
					push()
				}
			}

			if !openPairs && token != js.WhitespaceToken {
				prev = token
			}
		}
	}

	return keys
}

type Import struct {
	IsType     bool
	ExportName string
	LocalName  string
	Assertions string
}

type ImportStatement struct {
	Span       loc.Span
	Value      []byte
	IsType     bool
	Imports    []Import
	Specifier  string
	Assertions string
}

type ImportState uint32

const (
	ImportDefault ImportState = iota
	ImportNamed
)

func NextImportStatement(source []byte, pos int) (int, ImportStatement) {
	l := js.NewLexer(parse.NewInputBytes(source[pos:]))
	i := pos
	for {
		token, value := l.Next()

		if len(source) > i && token == js.DivToken || token == js.DivEqToken {
			lns := bytes.Split(source[i+1:], []byte{'\n'})
			if bytes.Contains(lns[0], []byte{'/'}) {
				token, value = l.RegExp()
			}
		}

		if token == js.ErrorToken {
			// EOF or other error
			return -1, ImportStatement{}
		}
		// Imports should be consumed up until we find a specifier,
		// then we can exit after the following line terminator or semicolon
		if token == js.ImportToken {
			i += len(value)
			text := []byte(value)
			isType := false
			specifier := ""
			assertion := ""
			foundSpecifier := false
			foundAssertion := false
			imports := make([]Import, 0)
			importState := ImportDefault
			currImport := Import{}
			pairs := make(map[byte]int)
			for {
				next, nextValue := l.Next()
				if len(source) > i && (next == js.DivToken || next == js.DivEqToken) {
					lns := bytes.Split(source[i+1:], []byte{'\n'})
					if bytes.Contains(lns[0], []byte{'/'}) {
						next, nextValue = l.RegExp()
					}
				}
				i += len(nextValue)
				text = append(text, nextValue...)

				if next == js.ErrorToken {
					break
				}

				if next == js.DotToken {
					isMeta := false
					for {
						next, _ := l.Next()
						if next == js.MetaToken {
							isMeta = true
						}
						if next != js.WhitespaceToken && next != js.MetaToken {
							break
						}
					}
					if isMeta {
						continue
					}
				}

				if !foundSpecifier && next == js.StringToken {
					if len(nextValue) > 1 {
						specifier = string(nextValue[1 : len(nextValue)-1])
						foundSpecifier = true
					}
					continue
				}

				if !foundSpecifier && next == js.IdentifierToken && string(nextValue) == "type" {
					isType = true
				}

				if foundSpecifier && (next == js.LineTerminatorToken || next == js.SemicolonToken) && pairs['{'] == 0 && pairs['('] == 0 && pairs['['] == 0 {
					if currImport.ExportName != "" {
						if currImport.LocalName == "" {
							currImport.LocalName = currImport.ExportName
						}
						imports = append(imports, currImport)
					}
					return i, ImportStatement{
						Span:       loc.Span{Start: i - len(text), End: i},
						Value:      text,
						IsType:     isType,
						Imports:    imports,
						Specifier:  specifier,
						Assertions: assertion,
					}
				}

				if next == js.WhitespaceToken {
					continue
				}

				if foundAssertion {
					assertion += string(nextValue)
				}

				if !foundAssertion && next == js.StringToken {
					specifier = string(nextValue[1 : len(nextValue)-1])
					foundSpecifier = true
					continue
				}

				if !foundAssertion && foundSpecifier && next == js.IdentifierToken && string(nextValue) == "assert" {
					foundAssertion = true
					continue
				}

				if !foundAssertion && next == js.OpenBraceToken {
					importState = ImportNamed
				}

				if !foundAssertion && next == js.CommaToken {
					if currImport.LocalName == "" {
						currImport.LocalName = currImport.ExportName
					}
					imports = append(imports, currImport)
					currImport = Import{}
				}

				if !foundAssertion && next == js.IdentifierToken {
					if currImport.ExportName != "" {
						currImport.LocalName = string(nextValue)
					} else if importState == ImportNamed {
						currImport.ExportName = string(nextValue)
					} else if importState == ImportDefault {
						currImport.ExportName = "default"
						currImport.LocalName = string(nextValue)
					}
				}

				if !foundAssertion && next == js.MulToken {
					currImport.ExportName = string(nextValue)
				}

				if js.IsPunctuator(next) {
					if nextValue[0] == '{' || nextValue[0] == '(' || nextValue[0] == '[' {
						pairs[nextValue[0]]++
					} else if nextValue[0] == '}' {
						pairs['{']--
					} else if nextValue[0] == ')' {
						pairs['(']--
					} else if nextValue[0] == ']' {
						pairs['[']--
					}
				}

				// do not hoist dynamic imports
				if next == js.OpenParenToken && len(specifier) == 0 {
					break
				}

				// do not hoist `{ import: "value" }`
				if next == js.ColonToken && len(specifier) == 0 {
					break
				}

				// if this is import.meta.*, ignore (watch for first dot)
				if next == js.DotToken && len(specifier) == 0 {
					break
				}
			}
		}

		i += len(value)
	}
}

/*
Determines the export name of a component, i.e. the object path to which
we can access the module, if it were imported using a dynamic import (`import()`)

Returns the export name and a boolean indicating whether
the component is imported AND used in the template.
*/
func ExtractComponentExportName(data string, imported Import) (string, bool) {
	namespacePrefix := fmt.Sprintf("%s.", imported.LocalName)
	isNamespacedComponent := strings.Contains(data, ".") && strings.HasPrefix(data, namespacePrefix)
	localNameEqualsData := imported.LocalName == data
	if isNamespacedComponent || localNameEqualsData {
		var exportName string
		switch true {
		case localNameEqualsData:
			exportName = imported.ExportName
		case imported.ExportName == "*":
			// matched a namespaced import
			exportName = strings.Replace(data, namespacePrefix, "", 1)
		case imported.ExportName == "default":
			// matched a default import
			exportName = strings.Replace(data, imported.LocalName, "default", 1)
		default:
			// matched a named import
			exportName = data
		}
		return exportName, true
	}
	return "", false
}
