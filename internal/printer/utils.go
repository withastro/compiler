package printer

import (
	"regexp"
	"strings"
)

func escapeText(src string) string {
	return escapeBackticks(
		escapeInterpolation(
			escapeExistingEscapes(src),
		),
	)
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
