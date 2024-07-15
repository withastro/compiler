import { type ParseResult, parse } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import type { FragmentNode } from '../../types.js';

const FIXTURE = '<>Hello</><Fragment>World</Fragment>';

let result: ParseResult;
test.before(async () => {
	result = await parse(FIXTURE);
});

test('fragment shorthand', () => {
	const [first] = result.ast.children as FragmentNode[];
	assert.equal(first.type, 'fragment', 'Expected first child to be of type "fragment"');
	assert.equal(first.name, '', 'Expected first child to have name of ""');
});

test('fragment literal', () => {
	const [, second] = result.ast.children as FragmentNode[];
	assert.equal(second.type, 'fragment', 'Expected second child to be of type "fragment"');
	assert.equal(second.name, 'Fragment', 'Expected second child to have name of ""');
});

test.run();
