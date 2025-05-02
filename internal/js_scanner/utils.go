package js_scanner

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/js"
)

const (
	gspIdent    = "getStaticPaths"
	importIdent = "import"
	exportIdent = "export"
)

func hasGetStaticPaths(exportBody []byte) bool {
	ident := []byte(gspIdent)
	return bytes.Contains(exportBody, ident)
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
							keys = append(keys, fmt.Appendf(nil, "%s: %s", value, ident))
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

	if !isNamespacedComponent && !localNameEqualsData {
		return "", false
	}

	var exportName string
	switch {
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
