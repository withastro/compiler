package test_utils

import (
	"fmt"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/lithammer/dedent"
)

func RemoveNewlines(input string) string {
	return strings.ReplaceAll(input, "\n", "")
}

func Dedent(input string) string {
	return dedent.Dedent( // removes any leading whitespace
		strings.ReplaceAll( // compress linebreaks to 1 or 2 lines max
			strings.TrimLeft(
				strings.TrimRight(input, " \n\r"), // remove any trailing whitespace
				" \t\r\n"),                        // remove leading whitespace
			"\n\n\n", "\n\n"),
	)
}

func ANSIDiff(x, y interface{}, opts ...cmp.Option) string {
	escapeCode := func(code int) string {
		return fmt.Sprintf("\x1b[%dm", code)
	}
	diff := cmp.Diff(x, y, opts...)
	if diff == "" {
		return ""
	}
	ss := strings.Split(diff, "\n")
	for i, s := range ss {
		switch {
		case strings.HasPrefix(s, "-"):
			ss[i] = escapeCode(31) + s + escapeCode(0)
		case strings.HasPrefix(s, "+"):
			ss[i] = escapeCode(32) + s + escapeCode(0)
		}
	}
	return strings.Join(ss, "\n")
}
