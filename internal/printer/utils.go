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
