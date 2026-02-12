import { type ParseResult, parse } from '@astrojs/compiler';
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';
import type { FragmentNode } from '../../../types.js';

const FIXTURE = '<>Hello</><Fragment>World</Fragment>';

describe('parse/fragment', { skip: true }, () => {
	let result: ParseResult;
	before(async () => {
		result = await parse(FIXTURE);
	});

	it('fragment shorthand', () => {
		const [first] = result.ast.children as FragmentNode[];
		assert.deepStrictEqual(first.type, 'fragment', 'Expected first child to be of type "fragment"');
		assert.deepStrictEqual(first.name, '', 'Expected first child to have name of ""');
	});

	it('fragment literal', () => {
		const [, second] = result.ast.children as FragmentNode[];
		assert.deepStrictEqual(
			second.type,
			'fragment',
			'Expected second child to be of type "fragment"',
		);
		assert.deepStrictEqual(second.name, 'Fragment', 'Expected second child to have name of ""');
	});
});
