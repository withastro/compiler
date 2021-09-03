package js_scanner

import "fmt"

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
var importStatements []*ImportStatement

// TODO: ignore `await` inside of function bodies
func FindRenderBody(_source []byte) int {
	source = _source
	pos = 0
	lastBr := 0
	for ; pos < len(source)-1; pos++ {
		c := readCommentWhitespace(false)
		switch true {
		case isBr(c) || c == ';':
			// Track the last position of a linebreak of ;
			// This is a rough proxy for "end of previous statement"
			lastBr = pos
		case c == 'A':
			// If we access the Astro global, we're in the function body
			if isKeywordStart() && str_eq5('A', 's', 't', 'r', 'o') {
				return lastBr + 1
			}
		case c == 'a':
			// If we have to await something, we're in the function body
			if isKeywordStart() && str_eq5('a', 'w', 'a', 'i', 't') {
				return lastBr + 1
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

func AccessesPrivateVars(_source []byte) bool {
	source = _source
	fmt.Println(string(source))
	pos = 0
	for ; pos < len(source)-1; pos++ {
		c := readCommentWhitespace(true)
		fmt.Println(string(c))
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

func addImport(statement_start int, start int, end int, Type ImportType) {
	statement := ImportStatement{
		Start:          start,
		End:            end,
		StatementStart: statement_start,
		StatementEnd:   end,
		Type:           Type,
	}
	importStatements = append(importStatements, &statement)
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

func str_eq4(c1 byte, c2 byte, c3 byte, c4 byte) bool {
	return len(source[pos:]) >= 4 && source[pos+3] == c4 && source[pos+2] == c3 && source[pos+1] == c2 && source[pos] == c1
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

func readLineComment() {
	for ; pos < len(source)-1; pos++ {
		c := source[pos]
		if c == '\n' || c == '\r' {
			return
		}
	}
}

func readImportStatement() {
	startPos := pos
	pos += 6

	if !isWsNotBr(source[pos]) {
		return
	}

	c := readCommentWhitespace(true)
	for ; pos < len(source)-1; pos++ {
		if c == '"' || c == '\'' || c == '{' || c == '*' {
			if c == '\'' || c == '"' {
				readImportString(startPos, c)
				return
			}
		}
		switch c {
		// // dynamic import
		// case '(':
		//   openTokenPosStack[openTokenDepth++] = startPos;
		//   if (*lastTokenPos == '.')
		//     return;
		//   // dynamic import indicated by positive d
		//   addImport(startPos, pos + 1, 0, startPos);
		//   cur_dynamic_import = import_write_head;
		//   // try parse a string, to record a safe dynamic import string
		//   pos++;
		//   ch = commentWhitespace(true);
		//   if (ch == '\'') {
		//     singleQuoteString();
		//   }
		//   else if (ch == '"') {
		//     doubleQuoteString();
		//   }
		//   else {
		//     pos--;
		//     return;
		//   }
		//   pos++;
		//   ch = commentWhitespace(true);
		//   if (ch == ',') {
		//     import_write_head->end = pos;
		//     pos++;
		//     ch = commentWhitespace(true);
		//     import_write_head->assert_index = pos;
		//     import_write_head->safe = true;
		//     pos--;
		//   }
		//   else if (ch == ')') {
		//     openTokenDepth--;
		//     import_write_head->end = pos;
		//     import_write_head->statement_end = pos;
		//     import_write_head->safe = true;
		//   }
		//   else {
		//     pos--;
		//   }
		//   return;
		// import.meta
		case '.':
			pos++
			c = readCommentWhitespace(true)
			// import.meta indicated by d == -2
			if c == 'm' && str_eq4('m', 'e', 't', 'a') && source[pos-1] != '.' {
				return
			}
			// addImport(startPos, startPos, pos + 4, IMPORT_META)
		default:
			c = readCommentWhitespace(true)
			continue
		}
	}
}

func readSingleQuoteString() {
	for ; pos < len(source)-1; pos++ {
		c := source[pos]
		if c == '\'' {
			return
		}
		if c == '\\' {
			pos++
			c = source[pos]
			if c == '\r' || c == '\n' {
				pos++
			}
		} else if isBr(c) {
			break
		}
	}
}

func readDoubleQuoteString() {
	for ; pos < len(source)-1; pos++ {
		c := source[pos]
		if c == '"' {
			return
		}
		if c == '\\' {
			pos++
			c = source[pos]
			if c == '\r' || c == '\n' {
				pos++
			}
		} else if isBr(c) {
			break
		}
	}
}

func readImportString(statement_start int, c byte) {
	startPos := pos + 1
	if c == '\'' {
		readSingleQuoteString()
	} else if c == '"' {
		readDoubleQuoteString()
	}
	pos++
	if source[pos] == ';' {
		pos++
	}
	addImport(statement_start, startPos, pos, StandardImport)

	// TODO: handle assert
	// ch = commentWhitespace(false)
	//   if (ch != 'a' || !str_eq6('a', 's', 's', 'e', 'r', 't')) {
	//     pos--;
	//     return;
	//   }
	//   char16_t* assertIndex = pos;
	//   pos += 6;
	//   ch = commentWhitespace(true);
	//   if (ch != '{') {
	//     pos = assertIndex;
	//     return;
	//   }
	//   const char16_t* assertStart = pos;
	//   do {
	//     pos++;
	//     ch = commentWhitespace(true);
	//     if (ch == '\'') {
	//       singleQuoteString();
	//       pos++;
	//       ch = commentWhitespace(true);
	//     }
	//     else if (ch == '"') {
	//       doubleQuoteString();
	//       pos++;
	//       ch = commentWhitespace(true);
	//     }
	//     else {
	//       ch = readToWsOrPunctuator(ch);
	//     }
	//     if (ch != ':') {
	//       pos = assertIndex;
	//       return;
	//     }
	//     pos++;
	//     ch = commentWhitespace(true);
	//     if (ch == '\'') {
	//       singleQuoteString();
	//     }
	//     else if (ch == '"') {
	//       doubleQuoteString();
	//     }
	//     else {
	//       pos = assertIndex;
	//       return;
	//     }
	//     pos++;
	//     ch = commentWhitespace(true);
	//     if (ch == ',') {
	//       pos++;
	//       ch = commentWhitespace(true);
	//       if (ch == '}')
	//         break;
	//       continue;
	//     }
	//     if (ch == '}')
	//       break;
	//     pos = assertIndex;
	//     return;
	//   } while (true);
	//   import_write_head->assert_index = assertStart;
	//   import_write_head->statement_end = pos + 1;
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
