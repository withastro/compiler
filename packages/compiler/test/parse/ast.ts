import { type ParseResult, parse } from '@astrojs/compiler';
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';
import type { ElementNode } from '../../types.js';

const FIXTURE = `
---
let value = 'world';
---

<h1 name="value" empty {shorthand} expression={true} literal=\`tags\`>Hello {value}</h1>
<div></div>
`;

describe('parse/ast', { skip: true }, () => {
	let result: ParseResult;
	before(async () => {
		result = await parse(FIXTURE);
	});

	it('ast', () => {
		assert.strictEqual(typeof result, 'object', `Expected "parse" to return an object!`);
		assert.deepStrictEqual(result.ast.type, 'root', `Expected "ast" root node to be of type "root"`);
	});

	it('frontmatter', () => {
		const [frontmatter] = result.ast.children;
		assert.deepStrictEqual(
			frontmatter.type,
			'frontmatter',
			`Expected first child node to be of type "frontmatter"`
		);
	});

	it('element', () => {
		const [, element] = result.ast.children;
		assert.deepStrictEqual(element.type, 'element', `Expected first child node to be of type "element"`);
	});

	it('element with no attributes', () => {
		const [, , , element] = result.ast.children as ElementNode[];
		assert.deepStrictEqual(element.attributes, [], `Expected the "attributes" property to be an empty array`);
	});

	it('element with no children', () => {
		const [, , , element] = result.ast.children as ElementNode[];
		assert.deepStrictEqual(element.children, [], `Expected the "children" property to be an empty array`);
	});
});
