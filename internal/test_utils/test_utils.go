package test_utils

import (
	"strings"

	"github.com/lithammer/dedent"
)

func Dedent(input string) string {
	return dedent.Dedent(strings.TrimLeft(input, " \t\r\n"))
}
