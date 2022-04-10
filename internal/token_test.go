package astro

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/withastro/compiler/internal/test_utils"
)

type TokenTypeTest struct {
	name     string
	input    string
	expected []TokenType
}

type TokenPanicTest struct {
	name    string
	input   string
	message string
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
			"dot component",
			`<pkg.Item>`,
			[]TokenType{StartTagToken},
		},
		{
			"noscript component",
			`<noscript><Component /></noscript>`,
			[]TokenType{StartTagToken, SelfClosingTagToken, EndTagToken},
		},
		{
			"end tag",
			`</html>`,
			[]TokenType{EndTagToken},
		},
		{
			"self-closing tag (slash)",
			`<meta charset="utf-8" />`,
			[]TokenType{SelfClosingTagToken},
		},
		{
			"self-closing title",
			`<title set:html={} /><div></div>`,
			[]TokenType{SelfClosingTagToken, StartTagToken, EndTagToken},
		},
		{
			"self-closing tag (no slash)",
			`<img width="480" height="320">`,
			[]TokenType{SelfClosingTagToken},
		},
		{
			"self-closing script",
			`<script />`,
			[]TokenType{SelfClosingTagToken},
		},
		{
			"self-closing script with sibling",
			`<script /><div></div><div />`,
			[]TokenType{SelfClosingTagToken, StartTagToken, EndTagToken, SelfClosingTagToken},
		},
		{
			"self-closing style",
			`<style />`,
			[]TokenType{SelfClosingTagToken},
		},
		{
			"self-closing style with sibling",
			`<style /><div></div><div />`,
			[]TokenType{SelfClosingTagToken, StartTagToken, EndTagToken, SelfClosingTagToken},
		},
		{
			"No expressions inside math",
			`<math>{test}</math>`,
			[]TokenType{StartTagToken, TextToken, TextToken, TextToken, EndTagToken},
		},
		{
			"No expressions inside math (complex)",
			`<math xmlns="http://www.w3.org/1998/Math/MathML"><annotation encoding="application/x-tex">\sqrt{2}</annotation></math>`,
			[]TokenType{StartTagToken, StartTagToken, TextToken, TextToken, TextToken, TextToken, EndTagToken, EndTagToken},
		},
		{
			"Expression attributes allowed inside math",
			`<math set:html={test} />`,
			[]TokenType{SelfClosingTagToken},
		},
		{
			"SVG (self-closing)",
			`<svg><path/></svg>`,
			[]TokenType{StartTagToken, SelfClosingTagToken, EndTagToken},
		},
		{
			"SVG (left open)",
			`<svg><path></svg>`, // note: this test isn’t “ideal” it’s just testing current behavior
			[]TokenType{StartTagToken, StartTagToken, EndTagToken},
		},
		{
			"SVG with style",
			`<svg><style>
				#fire {
					fill: orange;
					stroke: purple;
				}
				.wordmark {
					fill: black;
				}
		</style><path id="#fire" d="M0,0 M340,29"></path><path class="wordmark" d="M0,0 M340,29"></path></svg>`,
			[]TokenType{StartTagToken, StartTagToken, TextToken, EndTagToken, StartTagToken, EndTagToken, StartTagToken, EndTagToken, EndTagToken},
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
		{
			"expression with solidus inside element",
			`<div>{ 16 / 4 }</div>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"expression with strings inside element",
			`<div>{ "string" + 16 / 4 + "}" }</div>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, TextToken, TextToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"expression inside component",
			`<Component>{items.map(item => <div>{item}</div>)}</Component>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"expression inside component with quoted attr",
			`<Component a="b">{items.map(item => <div>{item}</div>)}</Component>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"expression inside component with expression attr",
			`<Component data={data}>{items.map(item => <div>{item}</div>)}</Component>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"expression inside component with named expression attr",
			`<Component named={data}>{items.map(item => <div>{item}</div>)}</Component>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"expression with multiple returns",
			`<div>{() => {
			let generate = (input) => {
				let a = () => { return; };
				let b = () => { return; };
				let c = () => { return; };
			};
		}}</div>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"attribute expression with quoted braces",
			`<div value={"{"} />`,
			[]TokenType{SelfClosingTagToken},
		},
		{
			"attribute expression with solidus",
			`<div value={100 / 2} />`,
			[]TokenType{SelfClosingTagToken},
		},
		{
			"attribute expression with solidus inside template literal",
			"<div value={attr ? `a/b` : \"c\"} />",
			[]TokenType{SelfClosingTagToken},
		},
		{
			"complex attribute expression",
			"<div value={`${attr ? `a/b ${`c ${`d ${cool}`}`}` : \"d\"} awesome`} />",
			[]TokenType{SelfClosingTagToken},
		},
		{
			"attribute expression with solidus no spaces",
			`<div value={(100/2)} />`,
			[]TokenType{SelfClosingTagToken},
		},
		{
			"attribute expression with quote",
			`<div value={/* hello */} />`,
			[]TokenType{SelfClosingTagToken},
		},
		{
			"JSX-style comment inside element",
			`<div {/* hello */} a=b />`,
			[]TokenType{SelfClosingTagToken},
		},
		{
			"quotes within textContent",
			`<p>can't</p>`,
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"apostrophe within title",
			`<title>Astro's</title>`,
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"quotes within title",
			`<title>My Astro "Website"</title>`,
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"textarea inside expression",
			`
						{bool && <textarea>It was a dark and stormy night...</textarea>}
						{bool && <input>}
					`,
			[]TokenType{StartExpressionToken, TextToken, StartTagToken, TextToken, EndTagToken, EndExpressionToken, TextToken, StartExpressionToken, TextToken, SelfClosingTagToken, EndExpressionToken, TextToken},
		},
		{
			"Markdown Inside markdown backtick treated as a string",
			"<Markdown>`{}`</Markdown>",
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"Quotes in elements in Markdown",
			"<Markdown><span>can't</span></Markdown>",
			[]TokenType{StartTagToken, StartTagToken, TextToken, EndTagToken, EndTagToken},
		},
		{
			"text containing a /",
			"<span>next/router</span>",
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"iframe allows attributes",
			"<iframe src=\"https://google.com\"></iframe>",
			[]TokenType{StartTagToken, EndTagToken},
		},
		{
			"data-astro-raw allows children to be parsed as Text",
			"<span data-astro-raw>function foo() { }</span>",
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"is:raw allows children to be parsed as Text",
			"<span is:raw>function foo() { }</span>",
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"is:raw treats all children as raw text",
			"<Fragment is:raw><ul></ue></Fragment>",
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"is:raw treats all children as raw text",
			"<Fragment is:raw><ul></ue></Fragment>",
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"is:raw allows other attributes",
			"<span data-raw={true} is:raw><%= Hi =%></span>",
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"Doesn't throw on other data attributes",
			"<span data-foo></span>",
			[]TokenType{StartTagToken, EndTagToken},
		},
		{
			"Doesn't work if attr is named data",
			"<span data>{Hello}</span>",
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"Supports <style> inside of <svg>",
			`<svg><style><div>:root { color: red; }</style></svg>`,
			[]TokenType{StartTagToken, StartTagToken, TextToken, EndTagToken, EndTagToken},
		},
		{
			"multiple scoped :global",
			`<style>:global(test-2) {}</style><style>test-1{}</style>`,
			[]TokenType{StartTagToken, TextToken, EndTagToken, StartTagToken, TextToken, EndTagToken},
		},
		{
			"multiple styles",
			`<style global>a {}</style><style>b {}</style><style>c {}</style>`,
			[]TokenType{StartTagToken, TextToken, EndTagToken, StartTagToken, TextToken, EndTagToken, StartTagToken, TextToken, EndTagToken},
		},
		{
			"element with single quote",
			`<div>Don't panic</div>`,
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"fragment",
			`<>foo</>`,
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"fragment shorthand",
			`<h1>A{cond && <>item <span>B{text}</span></>}</h1>`,
			[]TokenType{StartTagToken, TextToken, StartExpressionToken, TextToken, StartTagToken, TextToken, StartTagToken, TextToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, EndTagToken, EndExpressionToken, EndTagToken},
		},
		{
			"fragment",
			`<Fragment>foo</Fragment>`,
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"fragment shorthand in nested expression",
			`<div>{x.map((x) => (<>{x ? "truthy" : "falsy"}</>))}</div>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, StartTagToken, StartExpressionToken, TextToken, TextToken, EndExpressionToken, EndTagToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"select with expression",
			`<select>{[1, 2, 3].map(num => <option>{num}</option>)}</select>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"Markdown codeblock",
			fmt.Sprintf(`<Markdown>
		%s%s%s
		open brace {
		%s%s%s
		</Markdown>`, "`", "`", "`", "`", "`", "`"),
			[]TokenType{StartTagToken, TextToken, TextToken, TextToken, TextToken, EndTagToken},
		},
		{
			"Empty expression",
			"({})",
			[]TokenType{TextToken, StartExpressionToken, EndExpressionToken, TextToken},
		},
	}

	runTokenTypeTest(t, Basic)
}

func TestPanics(t *testing.T) {
	Panics := []TokenPanicTest{
		{
			"fragment with attributes",
			`< slot="named">foo</>`,
			`Unable to assign attributes when using <> Fragment shorthand syntax!

To fix this, please change
  < slot="named">
to use the longhand Fragment syntax:
  <Fragment slot="named">`,
		},
	}
	runPanicTest(t, Panics)
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
			"does not tokenize elements inside",
			`
			---
			const a = <div />;
			---
			`,
			[]TokenType{FrontmatterFenceToken, TextToken, TextToken, FrontmatterFenceToken},
		},
		{
			"no elements or expressions in frontmatter",
			`
			---
			const contents = "foo";
			const a = <div>{contents}</div>;
			---
			`,
			[]TokenType{FrontmatterFenceToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, FrontmatterFenceToken},
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
			[]TokenType{FrontmatterFenceToken, TextToken, TextToken, TextToken, TextToken, TextToken, FrontmatterFenceToken},
		},
		{
			"frontmatter tags and brackets all treated as text",
			`
			---
			const contents = "foo";
			const a = <ul>{contents}</ul>
			const someProps = {
				count: 0,
			}
			---
			`,
			[]TokenType{FrontmatterFenceToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, FrontmatterFenceToken},
		},
		{
			"less than isn’t a tag",
			`
			---
			const a = 2;
			const div = 4
			const isBigger = a < div;
			---
			`,
			[]TokenType{FrontmatterFenceToken, TextToken, FrontmatterFenceToken},
		},
		{
			"less than attr",
			`<div aria-hidden={count < 1} />`,
			[]TokenType{SelfClosingTagToken},
		},
		{
			"greater than attr",
			`<div aria-hidden={count > 1} />`,
			[]TokenType{SelfClosingTagToken},
		},
		{
			"greater than attr inside expression",
			`{values.map(value => <div aria-hidden={count > 1} />)}`,
			[]TokenType{StartExpressionToken, TextToken, SelfClosingTagToken, TextToken, EndExpressionToken},
		},
		{
			"single-line comments",
			`
			---
			// --- <div>
			---
			`,
			[]TokenType{FrontmatterFenceToken, TextToken, TextToken, FrontmatterFenceToken},
		},
		{
			"multi-line comments",
			`
			---
			/* --- <div> */
			---
			`,
			[]TokenType{FrontmatterFenceToken, TextToken, TextToken, FrontmatterFenceToken},
		},
		{
			"RegExp",
			`---
const RegExp = /---< > > { }; import thing from "thing"; /
---
			{html}`,
			[]TokenType{FrontmatterFenceToken, TextToken, TextToken, FrontmatterFenceToken, TextToken, StartExpressionToken, TextToken, EndExpressionToken},
		},
		{
			"RegExp with Escape",
			`---
export async function getStaticPaths() {
  const pattern = /\.md$/g;
}
---
<div />`,
			[]TokenType{FrontmatterFenceToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, FrontmatterFenceToken, SelfClosingTagToken},
		},
		{
			"textarea",
			`<textarea>{html}</textarea>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken},
		},
		// {
		// 	"less than with no space isn’t a tag",
		// 	`
		// 	---
		// 	const a = 2;
		// 	const div = 4
		// 	const isBigger = a <div
		// 	---
		// 	`,
		// 	[]TokenType{FrontmatterFenceToken, TextToken, FrontmatterFenceToken},
		// },
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
			"string expression",
			`{"<div {attr} />"}`,
			[]TokenType{StartExpressionToken, TextToken, EndExpressionToken},
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
			"expression map",
			`<div>
			  {items.map((item) => (
		      // < > < }
		      <div>{item}</div>
		    ))}
		  </div>`,
			[]TokenType{StartTagToken, TextToken, StartExpressionToken, TextToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, TextToken, EndExpressionToken, TextToken, EndTagToken},
		},
		{
			"left bracket within string",
			`{"{"}`,
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
			"expression within single-line comment",
			`{ // < > < }
		    'text'
		  }`,
			[]TokenType{StartExpressionToken, TextToken, TextToken, TextToken, EndExpressionToken},
		},
		{
			"expression within multi-line comment",
			`{/* < > < } */ 'text'}`,
			[]TokenType{StartExpressionToken, TextToken, TextToken, EndExpressionToken},
		},
		{
			"expression with nested strings",
			"{`${`${`${foo}`}`}`}",
			[]TokenType{StartExpressionToken, TextToken, TextToken, TextToken, TextToken, TextToken, EndExpressionToken},
		},
		{
			"element with multiple expressions",
			"<div>Hello {first} {last}</div>",
			[]TokenType{StartTagToken, TextToken, StartExpressionToken, TextToken, EndExpressionToken, TextToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"ternary render",
			"{false ? <div>#f</div> : <div>#t</div>}",
			[]TokenType{StartExpressionToken, TextToken, StartTagToken, TextToken, EndTagToken, TextToken, StartTagToken, TextToken, EndTagToken, EndExpressionToken},
		},
		{
			"title",
			"<title>test {expr} test</title>",
			[]TokenType{StartTagToken, TextToken, StartExpressionToken, TextToken, EndExpressionToken, TextToken, EndTagToken},
		},
		{
			"String interpolation inside an expression within a title",
			"<title>{content.title && `${title} 🚀 ${title}`}</title>",
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"Nested use of string templates inside expressions",
			"<div>{`${a} inner${a > 1 ? 's' : ''}.`}</div>",
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"expression with single quote",
			`{true && <div>Don't panic</div>}`,
			[]TokenType{StartExpressionToken, TextToken, StartTagToken, TextToken, EndTagToken, EndExpressionToken},
		},
		{
			"expression with double quote",
			`{true && <div>Don't panic</div>}`,
			[]TokenType{StartExpressionToken, TextToken, StartTagToken, TextToken, EndTagToken, EndExpressionToken},
		},
		{
			"expression with literal quote",
			`{true && <div>Don` + "`" + `t panic</div>}`,
			[]TokenType{StartExpressionToken, TextToken, StartTagToken, TextToken, EndTagToken, EndExpressionToken},
		},
		{
			"ternary expression with single quote",
			`{true ? <div>Don't panic</div> : <div>Do' panic</div>}`,
			[]TokenType{StartExpressionToken, TextToken, StartTagToken, TextToken, EndTagToken, TextToken, StartTagToken, TextToken, EndTagToken, EndExpressionToken},
		},
		{
			"single quote after expression",
			`{true && <div>{value} Don't panic</div>}`,
			[]TokenType{StartExpressionToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, TextToken, EndTagToken, EndExpressionToken},
		},
		{
			"single quote after self-closing",
			`{true && <div><span /> Don't panic</div>}`,
			[]TokenType{StartExpressionToken, TextToken, StartTagToken, SelfClosingTagToken, TextToken, EndTagToken, EndExpressionToken},
		},
		{
			"single quote after end tag",
			`{true && <div><span></span> Don't panic</div>}`,
			[]TokenType{StartExpressionToken, TextToken, StartTagToken, StartTagToken, EndTagToken, TextToken, EndTagToken, EndExpressionToken},
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
			"expression with apostrophe",
			`<div a="fred's" />`,
			[]AttributeType{QuotedAttribute},
		},
		{
			"shorthand",
			`<div {value} />`,
			[]AttributeType{ShorthandAttribute},
		},
		{
			"less than expression",
			`<div a={a < b} />`,
			[]AttributeType{ExpressionAttribute},
		},
		{
			"greater than expression",
			`<div a={a > b} />`,
			[]AttributeType{ExpressionAttribute},
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
		{
			"expression with quoted braces",
			`<div value={ "{" } />`,
			[]AttributeType{ExpressionAttribute},
		},
		{
			"attribute expression with solidus inside template literal",
			"<div value={attr ? `a/b` : \"c\"} />",
			[]AttributeType{ExpressionAttribute},
		},
		{
			"attribute expression with solidus inside template literal with trailing text",
			"<div value={`${attr ? `a/b` : \"c\"} awesome`} />",
			[]AttributeType{ExpressionAttribute},
		},
		{
			"iframe allows attributes",
			"<iframe src=\"https://google.com\"></iframe>",
			[]AttributeType{QuotedAttribute},
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

func runPanicTest(t *testing.T, suite []TokenPanicTest) {
	for _, tt := range suite {
		value := test_utils.Dedent(tt.input)
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewTokenizer(strings.NewReader(value))
			defer func() {
				r := recover()

				if r == nil {
					t.Errorf("%s did not panic\nExpected %s", tt.name, tt.message)
				}

				if diff := test_utils.ANSIDiff(test_utils.Dedent(r.(string)), test_utils.Dedent(tt.message)); diff != "" {
					t.Error(fmt.Sprintf("mismatch (-want +got):\n%s", diff))
				}
			}()
			var next TokenType
			for {
				next = tokenizer.Next()
				if next == ErrorToken {
					break
				}
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
