import assert from 'node:assert/strict';
import { before, describe, it } from 'node:test';
import { type ParseResult, parse } from '@astrojs/compiler';

const FIXTURE = `---
const name = "World";
---
<h1>Hello {name}!</h1>
<div class="container">
  <p>Some text</p>
</div>
<!-- a comment -->
`;

let result: ParseResult;

describe('parse', () => {
	before(async () => {
		result = await parse(FIXTURE);
	});

	it('returns AstroRoot', () => {
		assert.equal(result.ast.type, 'AstroRoot');
	});

	it('has no errors', () => {
		assert.equal(result.diagnostics.length, 0);
	});

	it('has frontmatter', () => {
		assert.ok(result.ast.frontmatter);
		assert.equal(result.ast.frontmatter.type, 'AstroFrontmatter');
	});

	it('frontmatter contains a program', () => {
		const program = result.ast.frontmatter.program;
		assert.equal(program.type, 'Program');
		assert.ok(program.body.length > 0);
	});

	it('body contains elements, text, and comments', () => {
		const types = result.ast.body.map((n: any) => n.type);
		assert.ok(types.includes('JSXElement'));
		assert.ok(types.includes('JSXText'));
		assert.ok(types.includes('AstroComment'));
	});

	it('elements have names', () => {
		const elements = result.ast.body.filter((n: any) => n.type === 'JSXElement');
		const names = elements.map((el: any) => el.openingElement.name.name);
		assert.ok(names.includes('h1'));
		assert.ok(names.includes('div'));
	});

	it('elements have attributes', () => {
		const div = result.ast.body.find(
			(n: any) => n.type === 'JSXElement' && n.openingElement.name.name === 'div',
		);
		assert.ok(div);
		assert.equal(div.openingElement.attributes.length, 1);
		assert.equal(div.openingElement.attributes[0].name.name, 'class');
	});

	it('elements have children', () => {
		const div = result.ast.body.find(
			(n: any) => n.type === 'JSXElement' && n.openingElement.name.name === 'div',
		);
		assert.ok(div);
		// <p> plus surrounding whitespace text nodes
		assert.ok(div.children.length > 0);
		const p = div.children.find(
			(n: any) => n.type === 'JSXElement' && n.openingElement.name.name === 'p',
		);
		assert.ok(p);
	});

	it('comment has value', () => {
		const comment = result.ast.body.find((n: any) => n.type === 'AstroComment');
		assert.ok(comment);
		assert.equal(comment.value, ' a comment ');
	});

	it('expression containers are parsed', () => {
		const h1 = result.ast.body.find(
			(n: any) => n.type === 'JSXElement' && n.openingElement.name.name === 'h1',
		);
		assert.ok(h1);
		const expr = h1.children.find((n: any) => n.type === 'JSXExpressionContainer');
		assert.ok(expr);
	});

	it('nodes have start/end positions', () => {
		assert.equal(typeof result.ast.start, 'number');
		assert.equal(typeof result.ast.end, 'number');
		assert.ok(result.ast.frontmatter.start >= 0);
		assert.ok(result.ast.frontmatter.end > result.ast.frontmatter.start);
	});
});

describe('parse: doctype', () => {
	it('parses doctype', async () => {
		const { ast } = await parse('<!doctype html>\n<html><head></head><body></body></html>');
		const doctype = ast.body.find((n: any) => n.type === 'AstroDoctype');
		assert.ok(doctype);
		assert.equal(doctype.value, 'html');
	});
});

describe('parse: fragments', () => {
	it('parses JSX fragments', async () => {
		const { ast } = await parse('<>Hello</>');
		const fragment = ast.body.find((n: any) => n.type === 'JSXFragment');
		assert.ok(fragment);
		assert.ok(fragment.children.length > 0);
	});
});

describe('parse: script tags', () => {
	it('parses script elements as JSXElement', async () => {
		const { ast } = await parse('<script>const x = 1;</script>');
		const script = ast.body.find(
			(n: any) => n.type === 'JSXElement' && n.openingElement.name.name === 'script',
		);
		assert.ok(script);
	});
});

describe('parse: empty file', () => {
	it('parses an empty file', async () => {
		const { ast, diagnostics } = await parse('');
		assert.equal(ast.type, 'AstroRoot');
		assert.equal(diagnostics.length, 0);
	});
});
