package astro

import (
	"reflect"
	"strings"
	"testing"
)

type TestCase struct {
	name  string
	input string
	want  []TokenType
}

func TestBasic(t *testing.T) {
	Basic := []TestCase{
		{
			"doctype",
			`<!DOCTYPE html>`,
			[]TokenType{DoctypeToken},
		},
		{
			"start tag",
			`<html>`,
			[]TokenType{StartTagToken},
		},
		{
			"end tag",
			`</html>`,
			[]TokenType{EndTagToken},
		},
		{
			"self-closing tag",
			`<meta charset="utf-8" />`,
			[]TokenType{SelfClosingTagToken},
		},
		{
			"text",
			` `,
			[]TokenType{TextToken},
		},
		{
			"comment",
			`<!-- comment -->`,
			[]TokenType{CommentToken},
		},
		{
			"top-level expression",
			`{ value }`,
			[]TokenType{StartExpressionToken, TextToken, EndExpressionToken},
		},
		{
			"expression inside element",
			`<div>{ value }</div>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken},
		},
	}

	for _, tt := range Basic {
		t.Run(tt.name, func(t *testing.T) {
			tokens := make([]TokenType, 0)
			parser := NewTokenizer(strings.NewReader(tt.input))
			var next TokenType
			for {
				next = parser.Next()
				if next == ErrorToken {
					break
				}
				tokens = append(tokens, next)
			}
			if !reflect.DeepEqual(tokens, tt.want) {
				t.Errorf("NewTokenizer() = %v, want %v", tokens, tt.want)
			}
		})
	}
}
