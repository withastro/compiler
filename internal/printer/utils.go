package printer

import (
	"regexp"
	"strings"

	"github.com/iancoleman/strcase"
)

func escapeText(src string) string {
	return escapeBackticks(
		escapeInterpolation(
			escapeExistingEscapes(src),
		),
	)
}

func escapeBraces(src string) string {
	return escapeTSXExpressions(
		escapeExistingEscapes(src),
	)
}

func getTSXComponentName(filename string) string {
	if filename == "<stdin>" {
		return "__AstroComponent_"
	}
	if len(filename) == 0 {
		return "__AstroComponent_"
	}
	parts := strings.Split(filename, "/")
	part := parts[len(parts)-1]
	if len(part) == 0 {
		return "__AstroComponent_"
	}
	basename := strcase.ToCamel(strings.Split(part, ".")[0])
	return strings.Join([]string{basename, "__AstroComponent_"}, "")
}

func getComponentName(pathname string) string {
	if len(pathname) == 0 {
		return "$$Component"
	}
	parts := strings.Split(pathname, "/")
	part := parts[len(parts)-1]
	if len(part) == 0 {
		return "$$Component"
	}
	basename := strcase.ToCamel(strings.Split(part, ".")[0])
	if basename == "Astro" {
		return "$$Component"
	}
	return strings.Join([]string{"$$", basename}, "")
}

func escapeExistingEscapes(src string) string {
	return strings.Replace(src, "\\", "\\\\", -1)
}

func escapeTSXExpressions(src string) string {
	open := regexp.MustCompile(`{`)
	close := regexp.MustCompile(`}`)
	return close.ReplaceAllString(open.ReplaceAllString(src, `\\{`), `\\}`)
}

func escapeInterpolation(src string) string {
	interpolation := regexp.MustCompile(`\${`)
	return interpolation.ReplaceAllString(src, "\\${")
}

// Escape backtick characters for Text nodes
func escapeBackticks(src string) string {
	backticks := regexp.MustCompile("`")
	return backticks.ReplaceAllString(src, "\\`")
}

func escapeSingleQuote(str string) string {
	return strings.Replace(str, "'", "\\'", -1)
}

func encodeDoubleQuote(str string) string {
	return strings.Replace(str, `"`, "&quot;", -1)
}

// Remove comment blocks from string (e.g. "/* a comment */aProp" => "aProp")
func removeComments(input string) string {
	var (
		sb        = strings.Builder{}
		inComment = false
	)
	for cur := 0; cur < len(input); cur++ {
		peekIs := func(assert byte) bool { return cur+1 < len(input) && input[cur+1] == assert }
		if input[cur] == '/' && !inComment && peekIs('*') {
			inComment = true
			cur++
		} else if input[cur] == '*' && inComment && peekIs('/') {
			inComment = false
			cur++
		} else if !inComment {
			sb.WriteByte(input[cur])
		}
	}

	if inComment {
		panic("unterminated comment")
	}

	return strings.TrimSpace(sb.String())
}
