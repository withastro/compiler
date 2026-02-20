import { type ParseResult, parse } from '@astrojs/compiler-rs';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

describe('bad-styles/unclosed-style', () => {
	it('can compile unfinished style', async () => {
		let error = 0;
		let result: ParseResult = {} as ParseResult;

		try {
			result = await parse('<style>');
		} catch (_e) {
			error = 1;
		}

		assert.deepStrictEqual(error, 0, 'Expected to compile with unfinished style.');
		assert.ok(result.ast, 'Expected to compile with unfinished style.');
		const style = (result.ast as any).body[0];
		assert.ok(style, 'Expected a style element in the AST body.');
		assert.deepStrictEqual(
			style.openingElement.name.name,
			'style',
			'Expected to compile with unfinished style.',
		);
	});
});
