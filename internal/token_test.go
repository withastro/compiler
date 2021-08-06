package astro

import (
	"reflect"
	"strings"
	"testing"
)

func TestTokenizer(t *testing.T) {
	tests := []struct {
		name string
		input string
		want []TokenType
	}{
		{
		  "doctype",
			`<!DOCTYPE html>`,
			[] TokenType { DoctypeToken },
		},
		{
		  "start tag",
			`<html>`,
			[] TokenType { StartTagToken },
		},
		{
		  "end tag",
			`</html>`,
			[] TokenType { EndTagToken },
		},
		{
		  "self-closing tag",
			`<meta charset="utf-8" />`,
			[] TokenType { SelfClosingTagToken },
		},
		{
		  "text",
			` `,
			[] TokenType { TextToken },
		},
		{
			"comment",
			`<!-- comment -->`,
			[] TokenType { CommentToken },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := make([]TokenType, 0)
			parser := NewTokenizer(strings.NewReader(tt.input))
			var next TokenType
			for {
				next = parser.Next()
				if (next == ErrorToken) {
					break
				}
				tokens = append(tokens, next)
			}
			if (!reflect.DeepEqual(tokens, tt.want)) {
				t.Errorf("NewTokenizer() = %v, want %v", tokens, tt.want)
			}
		})
	}
}
