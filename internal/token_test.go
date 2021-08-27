package astro

import (
	"reflect"
	"strings"
	"testing"

	"github.com/snowpackjs/astro/internal/test_utils"
)

type TokenTypeTest struct {
	name     string
	input    string
	expected []TokenType
}

type AttributeTest struct {
	name     string
	input    string
	expected []AttributeType
}

func TestBasic(t *testing.T) {
	Basic := []TokenTypeTest{
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
			"test",
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

	runTokenTypeTest(t, Basic)
}

func TestFrontmatter(t *testing.T) {
	Frontmatter := []TokenTypeTest{
		{
			"simple token",
			`---`,
			[]TokenType{FrontmatterFenceToken},
		},
		{
			"basic case",
			`
			---
			const a = 0;
			---
			`,
			[]TokenType{FrontmatterFenceToken, TextToken, FrontmatterFenceToken},
		},
		{
			"ignores leading whitespace",
			`

			---
			const a = 0;
			---
			`,
			[]TokenType{FrontmatterFenceToken, TextToken, FrontmatterFenceToken},
		},
		{
			"allows leading comments",
			`
			<!-- Why? Who knows! -->
			---
			const a = 0;
			---
			`,
			[]TokenType{CommentToken, FrontmatterFenceToken, TextToken, FrontmatterFenceToken},
		},
		{
			"treated as text after element",
			`
			<div />

			---
			const a = 0;
			---
			`,
			[]TokenType{SelfClosingTagToken, TextToken},
		},
		{
			"treated as text after closed",
			`
			---
			const a = 0;
			---
			<div>
			---
			</div>
			`,
			[]TokenType{FrontmatterFenceToken, TextToken, FrontmatterFenceToken, TextToken, StartTagToken, TextToken, EndTagToken, TextToken},
		},
		{
			"tokenizes elements inside",
			`
			---
			const a = <div />;
			---
			`,
			[]TokenType{FrontmatterFenceToken, TextToken, SelfClosingTagToken, TextToken, FrontmatterFenceToken},
		},
		{
			"brackets within frontmatter treated as text",
			`
			---
			const someProps = {
				count: 0,
			}
			---
			`,
			[]TokenType{FrontmatterFenceToken, TextToken, FrontmatterFenceToken},
		},
	}

	runTokenTypeTest(t, Frontmatter)
}

func TestExpressions(t *testing.T) {
	Expressions := []TokenTypeTest{
		{
			"simple expression",
			`{value}`,
			[]TokenType{StartExpressionToken, TextToken, EndExpressionToken},
		},
		{
			"object expression",
			`{{ value }}`,
			[]TokenType{StartExpressionToken, TextToken, TextToken, TextToken, EndExpressionToken},
		},
		{
			"tag expression",
			`{<div />}`,
			[]TokenType{StartExpressionToken, SelfClosingTagToken, EndExpressionToken},
		},
		{
			"function expression",
			`{() => {
				return value
			}}`,
			[]TokenType{StartExpressionToken, TextToken, TextToken, TextToken, TextToken, EndExpressionToken},
		},
		{
			"nested one level",
			`{() => {
				return <div>{value}</div>
			}}`,
			[]TokenType{StartExpressionToken, TextToken, TextToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, TextToken, TextToken, EndExpressionToken},
		},
		{
			"nested two levels",
			`{() => {
				return <div>{() => {
					return value
				}}</div>
			}}`,
			[]TokenType{StartExpressionToken, TextToken, TextToken, TextToken, StartTagToken, StartExpressionToken, TextToken, TextToken, TextToken, TextToken, EndExpressionToken, EndTagToken, TextToken, TextToken, EndExpressionToken},
		},
		{
			"nested two levels with tag",
			`{() => {
				return <div>{() => {
					return <div>{value}</div>
				}}</div>
			}}`,
			[]TokenType{StartExpressionToken, TextToken, TextToken, TextToken, StartTagToken, StartExpressionToken, TextToken, TextToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, TextToken, TextToken, EndExpressionToken, EndTagToken, TextToken, TextToken, EndExpressionToken},
		},
		{
			"left bracket within string",
			`{'{'}`,
			[]TokenType{StartExpressionToken, TextToken, EndExpressionToken},
		},
		{
			"right bracket within string",
			`{'}'}`,
			[]TokenType{StartExpressionToken, TextToken, EndExpressionToken},
		},
		{
			"expression within string",
			`{'{() => <Component />}'}`,
			[]TokenType{StartExpressionToken, TextToken, EndExpressionToken},
		},
		{
			"expression with nested strings",
			"{`${`${`${foo}`}`}`}",
			[]TokenType{StartExpressionToken, TextToken, EndExpressionToken},
		},
	}

	runTokenTypeTest(t, Expressions)
}

func TestAttributes(t *testing.T) {
	Attributes := []AttributeTest{
		{
			"double quoted",
			`<div a="value" />`,
			[]AttributeType{QuotedAttribute},
		},
		{
			"single quoted",
			`<div a='value' />`,
			[]AttributeType{QuotedAttribute},
		},
		{
			"not quoted",
			`<div a=value />`,
			[]AttributeType{QuotedAttribute},
		},
		{
			"expression",
			`<div a={value} />`,
			[]AttributeType{ExpressionAttribute},
		},
		{
			"shorthand",
			`<div {value} />`,
			[]AttributeType{ShorthandAttribute},
		},
		{
			"spread",
			`<div {...value} />`,
			[]AttributeType{SpreadAttribute},
		},
		{
			"template literal",
			"<div a=`value` />",
			[]AttributeType{TemplateLiteralAttribute},
		},
		{
			"all",
			"<div a='value' a={value} {value} {...value} a=`value` />",
			[]AttributeType{QuotedAttribute, ExpressionAttribute, ShorthandAttribute, SpreadAttribute, TemplateLiteralAttribute},
		},
		{
			"multiple quoted",
			`<div a="value" b='value' c=value/>`,
			[]AttributeType{QuotedAttribute, QuotedAttribute, QuotedAttribute},
		},
	}

	runAttributeTypeTest(t, Attributes)
}

func runTokenTypeTest(t *testing.T, suite []TokenTypeTest) {
	for _, tt := range suite {
		value := test_utils.Dedent(tt.input)
		t.Run(tt.name, func(t *testing.T) {
			tokens := make([]TokenType, 0)
			tokenizer := NewTokenizer(strings.NewReader(value))
			var next TokenType
			for {
				next = tokenizer.Next()
				if next == ErrorToken {
					break
				}
				tokens = append(tokens, next)
			}
			if !reflect.DeepEqual(tokens, tt.expected) {
				t.Errorf("Tokens = %v\nExpected = %v", tokens, tt.expected)
			}
		})
	}
}

func runAttributeTypeTest(t *testing.T, suite []AttributeTest) {
	for _, tt := range suite {
		value := test_utils.Dedent(tt.input)
		t.Run(tt.name, func(t *testing.T) {
			attributeTypes := make([]AttributeType, 0)
			tokenizer := NewTokenizer(strings.NewReader(value))
			var next TokenType
			for {
				next = tokenizer.Next()
				if next == ErrorToken {
					break
				}

				for _, attr := range tokenizer.Token().Attr {
					attributeTypes = append(attributeTypes, attr.Type)
				}
			}
			if !reflect.DeepEqual(attributeTypes, tt.expected) {
				t.Errorf("Attributes = %v\nExpected = %v", attributeTypes, tt.expected)
			}
		})
	}
}
