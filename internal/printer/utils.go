package printer

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/iancoleman/strcase"
	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/js_scanner"
	"github.com/withastro/compiler/internal/transform"
)

func escapeText(src string) string {
	return escapeBackticks(
		escapeInterpolation(
			escapeExistingEscapes(src),
		),
	)
}

func escapeBraces(src string) string {
	return escapeStarSlash(escapeTSXExpressions(
		escapeExistingEscapes(src),
	))
}

func escapeStarSlash(src string) string {
	return strings.ReplaceAll(src, "*/", "*\\/")
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
	if js_scanner.IsIdentifier([]byte(basename)) {
		return fmt.Sprintf("%s%s", basename, "__AstroComponent_")
	} else {
		return "__AstroComponent_"
	}
}

func trimExt(filename string) string {
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

func getParamsTypeFromFilename(filename string) string {
	defaultType := "Record<string, string | number>"
	if filename == "<stdin>" {
		return defaultType
	}
	if len(filename) == 0 {
		return defaultType
	}
	parts := strings.Split(filename, "/")
	params := make([]string, 0)
	r, err := regexp.Compile(`\[(?:\.{3})?([^]]+)\]`)
	if err != nil {
		return defaultType
	}
	for _, part := range parts {
		if !strings.ContainsAny(part, "[]") {
			continue
		}
		part = trimExt(part)
		for _, match := range r.FindAllStringSubmatch(part, -1) {
			params = append(params, fmt.Sprintf(`"%s"`, match[1]))
		}
	}
	if len(params) == 0 {
		return defaultType
	}
	return fmt.Sprintf("Record<%s, string | number>", strings.Join(params, " | "))
}

func getComponentName(filename string) string {
	if len(filename) == 0 {
		return "$$Component"
	}
	parts := strings.Split(filename, "/")
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

func escapeDoubleQuote(str string) string {
	return strings.Replace(str, `"`, "\\\"", -1)
}

func encodeDoubleQuote(str string) string {
	return strings.Replace(str, `"`, "&quot;", -1)
}

func convertAttributeValue(n *astro.Node, attrName string) string {
	expr := `""`
	if transform.HasAttr(n, attrName) {
		attr := transform.GetAttr(n, attrName)
		switch attr.Type {
		case astro.QuotedAttribute:
			expr = fmt.Sprintf(`"%s"`, attr.Val)
		case astro.ExpressionAttribute:
			expr = fmt.Sprintf(`(%s)`, attr.Val)
		case astro.TemplateLiteralAttribute:
			expr = fmt.Sprintf("`%s`", attr.Val)
		}
	}
	return expr
}
