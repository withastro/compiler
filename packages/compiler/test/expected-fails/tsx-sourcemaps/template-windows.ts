import { convertToTSX } from '@astrojs/compiler-rs';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testTSXSourcemap } from '../../utils.js';

describe('tsx-sourcemaps/template-windows', { skip: true }, () => {
	it('last character does not end up in middle of CRLF', async () => {
		const input = "---\r\nimport { Meta } from '$lib/components/Meta.astro';\r\n---\r\n";
		const output = await testTSXSourcemap(input, ';');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 2,
			column: 50,
			name: null,
		});
	});

	it('template expression basic', async () => {
		const input = '<div>{\r\nnonexistent\r\n}</div>';

		const output = await testTSXSourcemap(input, 'nonexistent');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 2,
			column: 1,
			name: null,
		});
	});

	it('template expression has dot', async () => {
		const input = '<div>{\nconsole.log(hey)\n}</div>';
		const output = await testTSXSourcemap(input, 'log');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 2,
			column: 9,
			name: null,
		});
	});

	it('template expression has dot', async () => {
		const input = '<div>{\r\nconsole.log(hey)\r\n}</div>';
		const output = await testTSXSourcemap(input, 'log');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 2,
			column: 9,
			name: null,
		});
	});

	it('template expression with addition', async () => {
		const input = `{"hello" + \nhey}`;
		const output = await testTSXSourcemap(input, 'hey');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 2,
			column: 1,
			name: null,
		});
	});

	it('template expression with addition', async () => {
		const input = `{"hello" + \r\nhey}`;
		const output = await testTSXSourcemap(input, 'hey');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 2,
			column: 1,
			name: null,
		});
	});

	it('html attribute', async () => {
		const input = `<svg\nvalue="foo" color="#000"></svg>`;
		const output = await testTSXSourcemap(input, 'color');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			name: null,
			line: 2,
			column: 12,
		});
	});

	it('html attribute', async () => {
		const input = `<svg\r\nvalue="foo" color="#000"></svg>`;
		const output = await testTSXSourcemap(input, 'color');
		assert.deepStrictEqual(output, {
			source: 'index.astro',
			name: null,
			line: 2,
			column: 12,
		});
	});

	it('complex template expression', async () => {
		const input = `{[].map(ITEM => {\r\nv = "what";\r\nreturn <div>{ITEMS}</div>\r\n})}`;
		const item = await testTSXSourcemap(input, 'ITEM');
		const items = await testTSXSourcemap(input, 'ITEMS');
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
		const input = `<div\r\na="b" className="hello" />`;
		const className = await testTSXSourcemap(input, 'className');
		assert.deepStrictEqual(className, {
			source: 'index.astro',
			name: null,
			line: 2,
			column: 6,
		});
	});

	it('special attributes', async () => {
		const input = `<div\r\na="b" @on.click="fn" />`;
		const onClick = await testTSXSourcemap(input, '@on.click');
		assert.deepStrictEqual(onClick, {
			source: 'index.astro',
			name: null,
			line: 2,
			column: 6,
		});
	});

	it('whitespace', async () => {
		const input = `---\r\nimport A from "a";\r\n\timport B from "b";\r\n---\r\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'both', filename: 'index.astro' });
		assert.ok(code.includes('\t'), 'output includes \\t');

		const B = await testTSXSourcemap(input, 'B');
		assert.deepStrictEqual(B, {
			source: 'index.astro',
			name: null,
			line: 3,
			column: 9,
		});
	});
});
