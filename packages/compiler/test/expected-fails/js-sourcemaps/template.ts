import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testJSSourcemap } from '../../utils.js';

describe('js-sourcemaps/template', { skip: true }, () => {
	it('template expression basic', async () => {
		const input = '<div>{nonexistent}</div>';

		const output = await testJSSourcemap(input, 'nonexistent');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 1,
			column: 6,
			name: null,
		});
	});

	it('template expression has dot', async () => {
		const input = '<div>{console.log(hey)}</div>';
		const output = await testJSSourcemap(input, 'log');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 1,
			column: 14,
			name: null,
		});
	});

	it('template expression with addition', async () => {
		const input = `{"hello" + hey}`;
		const output = await testJSSourcemap(input, 'hey');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 1,
			column: 11,
			name: null,
		});
	});

	it('html attribute', async () => {
		const input = `<svg color="#000"></svg>`;
		const output = await testJSSourcemap(input, 'color');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			name: null,
			line: 1,
			column: 5,
		});
	});

	it('complex template expression', async () => {
		const input = `{[].map(ITEM => {
v = "what";
return <div>{ITEMS}</div>
})}`;
		const item = await testJSSourcemap(input, 'ITEM');
		const items = await testJSSourcemap(input, 'ITEMS');
		assert.deepStrictEqual(item, {
			source: 'index.astro',
			name: null,
			line: 1,
			column: 8,
		});
		assert.deepStrictEqual(items, {
			source: 'index.astro',
			name: null,
			line: 3,
			column: 14,
		});
	});

	it('attributes', async () => {
		const input = `<div className="hello" />`;
		const className = await testJSSourcemap(input, 'className');
		assert.deepStrictEqual(className, {
			source: 'index.astro',
			name: null,
			line: 1,
			column: 5,
		});
	});

	it('special attributes', async () => {
		const input = `<div @on.click="fn" />`;
		const onClick = await testJSSourcemap(input, '@on.click');
		assert.deepStrictEqual(onClick, {
			source: 'index.astro',
			name: null,
			line: 1,
			column: 5,
		});
	});
});
