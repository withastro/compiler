package js_scanner

// An ImportType is the type of import.
type ImportType uint32

const (
	StandardImport ImportType = iota
	DynamicImport
)

type ImportStatement struct {
	Type           ImportType
	Start          int
	End            int
	StatementStart int
	StatementEnd   int
}

var source []byte
var pos int
var pairs map[byte]int
var flags map[string]bool
var templateStack int

func FindRenderBody(_source []byte) int {
	source = _source
	pos = 0
	pairs = make(map[byte]int, 0)
	flags = make(map[string]bool, 2)
	templateStack = 0
	lastBr := -1
	insideBlock := false

	for ; pos < len(source)-1; pos++ {
		c := readCommentWhitespace(false)
		if insideBlock && pairs['{'] == 0 && pairs['('] == 0 && pairs['['] == 0 {
			insideBlock = false
		}
		switch true {
		case isBr(c) || c == ';':
			// Track the last position of a linebreak of ;
			// This is a rough proxy for "end of previous statement"
			if insideBlock {
				insideBlock = false
				continue
			} else {
				lastBr = pos
			}
		case c == '\'' || c == '"':
			readStringLiteral(c)
			continue
		case c == '`':
			readTemplateString()
			continue
		case c == '{' || c == '(' || c == '[':
			pairs[c]++
		case c == '}':
			if pairs['{'] > 0 {
				pairs['{']--
			} else if templateStack > 0 {
				templateStack--
				if templateStack == 0 {
					readTemplateString()
				}
			}
			if insideBlock && pairs['{'] == 0 && pairs['('] == 0 {
				insideBlock = false
			}
			continue
		case c == ')':
			pairs[')']--
		case c == ']':
			pairs['[']--
		case c == '=':
			if str_eq2('=', '>') {
				insideBlock = true
				readShorthandFn()
				continue
			}
		case c == 'A':
			// If we access the Astro global, we're in the function body
			if !insideBlock && pairs['{'] == 0 && isKeywordStart() && str_eq5('A', 's', 't', 'r', 'o') {
				return lastBr + 1
			}
		case c == 'a':
			if !insideBlock && pairs['{'] == 0 && isKeywordStart() && str_eq5('a', 'w', 'a', 'i', 't') {
				return lastBr + 1
			}
			if isKeywordStart() && str_eq5('a', 's', 'y', 'n', 'c') {
				insideBlock = true
				continue
			}
		case c == '/':
			if str_eq2('/', '/') {
				readLineComment()
				continue
			} else if str_eq2('/', '*') {
				readBlockComment(true)
				continue
			}
		}
	}
	return -1
}

func HasExports(_source []byte) bool {
	source = _source
	pos = 0
	for ; pos < len(source)-1; pos++ {
		c := readCommentWhitespace(true)
		switch true {
		case c == 'e':
			if isKeywordStart() && str_eq6('e', 'x', 'p', 'o', 'r', 't') {
				return true
			}
		case c == '/':
			if str_eq2('/', '/') {
				readLineComment()
				continue
			} else if str_eq2('/', '*') {
				readBlockComment(true)
				continue
			}
		}
	}
	return false
}

// TODO: check for access to $$vars
func AccessesPrivateVars(_source []byte) bool {
	source = _source
	pos = 0
	for ; pos < len(source)-1; pos++ {
		c := readCommentWhitespace(true)
		switch true {
		// case c == '$':
		// 	fmt.Println(str_eq2('$', '$'))
		// 	if isKeywordStart() && str_eq2('$', '$') {
		// 		return true
		// 	}
		case c == '/':
			if str_eq2('/', '/') {
				readLineComment()
				continue
			} else if str_eq2('/', '*') {
				readBlockComment(true)
				continue
			}
		}
	}
	return false
}

// Note: non-asii BR and whitespace checks omitted for perf / footprint
// if there is a significant user need this can be reconsidered
func isBr(c byte) bool {
	return c == '\r' || c == '\n'
}

func isWsNotBr(c byte) bool {
	return c == 9 || c == 11 || c == 12 || c == 32 || c == 160
}

func isBrOrWs(c byte) bool {
	return c > 8 && c < 14 || c == 32 || c == 160
}

func isBrOrWsOrPunctuatorNotDot(c byte) bool {
	return c > 8 && c < 14 || c == 32 || c == 160 || isPunctuator(c) && c != '.'
}

func isPunctuator(ch byte) bool {
	// 23 possible punctuator endings: !%&()*+,-./:;<=>?[]^{}|~
	return ch == '!' || ch == '%' || ch == '&' ||
		ch > 39 && ch < 48 || ch > 57 && ch < 64 ||
		ch == '[' || ch == ']' || ch == '^' ||
		ch > 122 && ch < 127
}

func str_eq2(c1 byte, c2 byte) bool {
	return len(source[pos:]) >= 2 && source[pos+1] == c2 && source[pos] == c1
}

func str_eq5(c1 byte, c2 byte, c3 byte, c4 byte, c5 byte) bool {
	return len(source[pos:]) >= 5 && source[pos+4] == c5 && source[pos+3] == c4 && source[pos+2] == c3 && source[pos+1] == c2 && source[pos] == c1
}

func str_eq6(c1 byte, c2 byte, c3 byte, c4 byte, c5 byte, c6 byte) bool {
	return len(source[pos:]) >= 6 && source[pos+5] == c6 && source[pos+4] == c5 && source[pos+3] == c4 && source[pos+2] == c3 && source[pos+1] == c2 && source[pos] == c1
}

func isKeywordStart() bool {
	return isBrOrWsOrPunctuatorNotDot(source[pos-1])
}

func readBlockComment(br bool) {
	pos++
	for ; pos < len(source)-1; pos++ {
		c := source[pos]
		if !br && isBr(c) {
			return
		}
		if c == '*' && source[pos+1] == '/' {
			pos++
			return
		}
	}
}

func readShorthandFn() {
	for ; pos < len(source)-1; pos++ {
		c := source[pos]
		if c == '{' || c == '(' || c == '[' {
			pairs[c]++
			return
		} else if isBr(c) {
			return
		}
	}
}

func readLineComment() {
	for ; pos < len(source)-1; pos++ {
		c := source[pos]
		if c == '\n' || c == '\r' {
			return
		}
	}
}

func readTemplateString() {
	for ; pos < len(source)-1; pos++ {
		c := source[pos]
		if c == '$' && source[pos+1] == '{' {
			pos++
			templateStack++
			return
		}
		if c == '`' {
			return
		}
		if c == '\\' {
			pos++
		}
	}
}

func readStringLiteral(quote byte) {
	for ; pos < len(source)-1; pos++ {
		c := source[pos]
		if c == quote {
			return
		}

		if c == '\\' {
			pos++
			c = source[pos]
			if c == '\r' && source[pos+1] == '\n' {
				pos++
			}
		} else if isBr(c) {
			break
		}
	}
}

func readCommentWhitespace(br bool) byte {
	var c byte
	for ; pos < len(source)-1; pos++ {
		c = source[pos]
		switch true {
		case c == '/':
			if str_eq2('/', '/') {
				readLineComment()
				continue
			} else if str_eq2('/', '*') {
				readBlockComment(true)
				continue
			} else {
				return c
			}
		case (br && !isBrOrWs(c)):
			return c
		case (!br && !isWsNotBr(c)):
			return c
		}
	}
	return c
}
