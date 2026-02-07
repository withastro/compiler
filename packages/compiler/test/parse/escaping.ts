import { parse } from '@astrojs/compiler';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

const STYLE = 'div { & span { color: red; }}';
const FIXTURE = `<style>${STYLE}</style>`;

describe('parse/escaping', { skip: true }, () => {
	it('ampersand', async () => {
		const result = await parse(FIXTURE);
		assert.ok(result.ast, 'Expected an AST to be generated');
		const [
			{
				children: [{ value: output }],
			},
		] = result.ast.children as any;
		assert.deepStrictEqual(output, STYLE, 'Expected AST style to equal input');
	});
});
