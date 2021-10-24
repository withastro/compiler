package test_utils

import (
	"strings"

	"github.com/lithammer/dedent"
)

func Dedent(input string) string {
	return dedent.Dedent( // removes any leading whitespace
		strings.ReplaceAll( // compress linebreaks to 1 or 2 lines max
			strings.TrimLeft(
				strings.TrimRight(input, " \n\r"), // remove any trailing whitespace
				" \t\r\n"),                        // remove leading whitespace
			"\n\n\n", "\n\n"),
	)
}
