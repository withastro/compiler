package astro

import (
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

type AttributeTest struct {
	name     string
	input    string
	expected []AttributeType
}

type LocTest struct {
	name     string
	input    string
	expected []int
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
			"unclosed tag",
			`<components.`,
			[]TokenType{TextToken},
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
			"text",
			`Hello@`,
			[]TokenType{TextToken},
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
			"attribute with quoted template literal",
			"<a :href=\"`/home`\">Home</a>",
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"No expressions inside math",
			`<math>{test}</math>`,
			[]TokenType{StartTagToken, TextToken, TextToken, TextToken, EndTagToken},
		},
		{
			"No expressions inside math (complex)",
			`<span><math xmlns="http://www.w3.org/1998/Math/MathML"><mo>4</mo><mi /><semantics><annotation encoding="application/x-tex">\sqrt {x}</annotation></semantics></math></span>`,
			[]TokenType{StartTagToken, StartTagToken, StartTagToken, TextToken, EndTagToken, SelfClosingTagToken, StartTagToken, StartTagToken, TextToken, TextToken, TextToken, TextToken, EndTagToken, EndTagToken, EndTagToken, EndTagToken},
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
			"form element with expression follwed by another form",
			`<form>{data.formLabelA}</form><form><button></button></form>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, StartTagToken, StartTagToken, EndTagToken, EndTagToken},
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
			"expression with multiple elements",
			`<div>{() => {
				if (value > 0.25) {
					return <span>Default</span>
				} else if (value > 0.5) {
					return <span>Another</span>
				} else if (value > 0.75) {
					return <span>Other</span>
				}
				return <span>Yet Other</span>
			}}</div>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, TextToken, TextToken, TextToken, TextToken, StartTagToken, TextToken, EndTagToken, TextToken, TextToken, TextToken, TextToken, StartTagToken, TextToken, EndTagToken, TextToken, TextToken, TextToken, TextToken, StartTagToken, TextToken, EndTagToken, TextToken, TextToken, StartTagToken, TextToken, EndTagToken, TextToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"expression with multiple elements returning self closing tags",
			`<div>{()=>{
				if (true) {
					return <hr />;
				};
				if (true) {
					return <img />;
				}
			}}</div>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, TextToken, TextToken, TextToken, TextToken, SelfClosingTagToken, TextToken, TextToken, TextToken, TextToken, SelfClosingTagToken, TextToken, TextToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"expression returning a mix of self-closing tags and elements",
			`<div>{() => {
				if (value > 0.25) {
					return <br />
				} else if (value > 0.5) {
					return <hr />
				} else if (value > 0.75) {
					return <div />
				}
				return <div>Yaaay</div>
			}}</div>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, TextToken, TextToken, TextToken, TextToken, SelfClosingTagToken, TextToken, TextToken, TextToken, TextToken, SelfClosingTagToken, TextToken, TextToken, TextToken, TextToken, SelfClosingTagToken, TextToken, TextToken, StartTagToken, TextToken, EndTagToken, TextToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"expression with switch returning a mix of self-closing tags and elements",
			`<div>{items.map(({ type, ...data }) => { switch (type) { case 'card': { return (<Card {...data} />);}case 'paragraph': { return (<p>{data.body}</p>);}}})}</div>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, SelfClosingTagToken, TextToken, TextToken, TextToken, TextToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, TextToken, TextToken, TextToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"expression with < operators",
			`<div>{() => {
				if (value < 0.25) {
					return <span>Default</span>
				} else if (value <0.5) {
					return <span>Another</span>
				} else if (value < 0.75) {
					return <span>Other</span>
				}
				return <span>Yet Other</span>
			}}</div>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, TextToken, TextToken, TextToken, TextToken, StartTagToken, TextToken, EndTagToken, TextToken, TextToken, TextToken, TextToken, StartTagToken, TextToken, EndTagToken, TextToken, TextToken, TextToken, TextToken, StartTagToken, TextToken, EndTagToken, TextToken, TextToken, StartTagToken, TextToken, EndTagToken, TextToken, TextToken, EndExpressionToken, EndTagToken},
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
			`<h1>A{cond && <>item <span>{text}</span></>}</h1>`,
			[]TokenType{StartTagToken, TextToken, StartExpressionToken, TextToken, StartTagToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, EndTagToken, EndExpressionToken, EndTagToken},
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
			"select with expression",
			`<select>{[1, 2, 3].map(num => <option>{num}</option>)}</select><div>Hello</div>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, TextToken, EndExpressionToken, EndTagToken, StartTagToken, TextToken, EndTagToken},
		},
		{
			"single open brace",
			"<main id={`{`}></main>",
			[]TokenType{StartTagToken, EndTagToken},
		},
		{
			"single close brace",
			"<main id={`}`}></main>",
			[]TokenType{StartTagToken, EndTagToken},
		},
		{
			"extra close brace",
			"<main id={`${}}`}></main>",
			[]TokenType{StartTagToken, EndTagToken},
		},
		{
			"Empty expression",
			"({})",
			[]TokenType{TextToken, StartExpressionToken, EndExpressionToken, TextToken},
		},
		{
			"expression after text",
			`<h1>A{cond && <span>Test {text}</span>}</h1>`,
			[]TokenType{StartTagToken, TextToken, StartExpressionToken, TextToken, StartTagToken, TextToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, EndExpressionToken, EndTagToken},
		},
		{
			"expression surrounded by text",
			`<h1>A{cond && <span>Test {text} Cool</span>}</h1>`,
			[]TokenType{StartTagToken, TextToken, StartExpressionToken, TextToken, StartTagToken, TextToken, StartExpressionToken, TextToken, EndExpressionToken, TextToken, EndTagToken, EndExpressionToken, EndTagToken},
		},
		{
			"switch statement",
			`<div>{() => { switch(value) { case 'a': return <A></A>; case 'b': return <B />; case 'c': return <C></C> }}}</div>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, StartTagToken, EndTagToken, TextToken, TextToken, SelfClosingTagToken, TextToken, TextToken, StartTagToken, EndTagToken, TextToken, TextToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"switch statement with expression",
			`<div>{() => { switch(value) { case 'a': return <A>{value}</A>; case 'b': return <B />; case 'c': return <C>{value.map(i => <span>{i}</span>)}</C> }}}</div>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, TextToken, TextToken, TextToken, TextToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, TextToken, TextToken, SelfClosingTagToken, TextToken, TextToken, StartTagToken, StartExpressionToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, TextToken, EndExpressionToken, EndTagToken, TextToken, TextToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"attribute expression with unmatched quotes",
			"<h1 set:text={`Houston we've got a problem`}></h1>",
			[]TokenType{StartTagToken, EndTagToken},
		},
		{
			"attribute expression with unmatched quotes",
			"<h1 set:html={`Oh \"no...`}></h1>",
			[]TokenType{StartTagToken, EndTagToken},
		},
		{
			"attribute expression with unmatched quotes inside matched quotes",
			"<h1 set:html={\"hello y'all\"}></h1>",
			[]TokenType{StartTagToken, EndTagToken},
		},
		{
			"attribute expression with unmatched quotes inside matched quotes II",
			"<h1 set:html={'\"Did Nate handle this case, too?\", Fred pondered...'}></h1>",
			[]TokenType{StartTagToken, EndTagToken},
		},
		{
			"typescript generic",
			`<ul>{items.map((item: Item<Checkbox>)) => <li>{item.checked}</li>)}</ul>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, TextToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"typescript generic II",
			`<ul>{items.map((item: Item<Checkbox>)) => <Checkbox>{item.checked}</Checkbox>)}</ul>`,
			[]TokenType{StartTagToken, StartExpressionToken, TextToken, TextToken, TextToken, StartTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, TextToken, EndExpressionToken, EndTagToken},
		},
		{
			"incomplete tag",
			`<MyAstroComponent`,
			[]TokenType{TextToken},
		},
		{
			"incomplete tag II",
			`<MyAstroComponent` + "\n",
			[]TokenType{TextToken},
		},
		{
			"incomplete tag III",
			`<div></div><MyAstroComponent` + "\n",
			[]TokenType{StartTagToken, EndTagToken, TextToken},
		},
		{
			"incomplete tag IV",
			`<span>n < value</span>`,
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"incomplete tag V",
			`<span>n<value</span>`,
			[]TokenType{StartTagToken, TextToken, EndTagToken},
		},
		{
			"incomplete tag V",
			"<div class=\"name\"\n<h1 />",
			[]TokenType{StartTagToken, TextToken, SelfClosingTagToken},
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
			"nested one level with self-closing tag before expression",
			`{() => {
				return <div><div />{value}</div>
			}}`,
			[]TokenType{StartExpressionToken, TextToken, TextToken, TextToken, StartTagToken, SelfClosingTagToken, StartExpressionToken, TextToken, EndExpressionToken, EndTagToken, TextToken, TextToken, EndExpressionToken},
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
			"expression with template literal",
			"<div a=\"`value`\" />",
			[]AttributeType{QuotedAttribute},
		},
		{
			"expression with template literal interpolation",
			"<div a=\"`${value}`\" />",
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
		{
			"shorthand attribute with comment",
			"<div {/* a comment */ value} />",
			[]AttributeType{ShorthandAttribute},
		},
		{
			"expression with comment",
			"<div a={/* a comment */ value} />",
			[]AttributeType{ExpressionAttribute},
		},
	}

	runAttributeTypeTest(t, Attributes)
}

func TestLoc(t *testing.T) {
	Locs := []LocTest{
		{
			"doctype",
			`<!DOCTYPE html>`,
			[]int{0, 11},
		},
		{
			"frontmatter",
			`---
doesNotExist
---
`,
			[]int{0, 1, 4},
		},
		{
			"expression",
			`<div>{console.log(hey)}</div>`,
			[]int{0, 2, 6, 7, 23, 26},
		},
		{
			"expression II",
			`{"hello" + hey}`,
			[]int{0, 1, 2, 9, 15},
		},
		{
			"element I",
			`<div></div>`,
			[]int{0, 2, 8},
		},
	}

	runTokenLocTest(t, Locs)
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

func runTokenLocTest(t *testing.T, suite []LocTest) {
	for _, tt := range suite {
		value := test_utils.Dedent(tt.input)
		t.Run(tt.name, func(t *testing.T) {
			locs := make([]int, 0)
			tokenizer := NewTokenizer(strings.NewReader(value))
			var next TokenType
			locs = append(locs, tokenizer.Token().Loc.Start)
			for {
				next = tokenizer.Next()
				if next == ErrorToken {
					break
				}
				tok := tokenizer.Token()
				locs = append(locs, tok.Loc.Start+1)
			}
			if !reflect.DeepEqual(locs, tt.expected) {
				t.Errorf("Tokens = %v\nExpected = %v", locs, tt.expected)
			}
		})
	}
}
