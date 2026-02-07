import { type ParseResult, parse } from '@astrojs/compiler';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import type { ElementNode } from '../../types.js';

describe('bad-styles/unclosed-style', { skip: 'parse() not implemented' }, () => {
	it('can compile unfinished style', async () => {
		let error = 0;
		let result: ParseResult = {} as ParseResult;

		try {
			result = await parse('<style>');
		} catch (e) {
			error = 1;
		}

		const style = result.ast.children[0] as ElementNode;
		assert.deepStrictEqual(error, 0, 'Expected to compile with unfinished style.');
		assert.ok(result.ast, 'Expected to compile with unfinished style.');
		assert.deepStrictEqual(style.name, 'style', 'Expected to compile with unfinished style.');
	});
});
