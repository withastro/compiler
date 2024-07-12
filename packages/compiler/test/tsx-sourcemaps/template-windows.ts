import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testTSXSourcemap } from '../utils';

test('template expression basic', async () => {
	const input = '<div>{\r\nnonexistent\r\n}</div>';

	const output = await testTSXSourcemap(input, 'nonexistent');
	assert.equal(output, {
		source: 'index.astro',
		line: 2,
		column: 1,
		name: null,
	});
});

test('template expression has dot', async () => {
	const input = '<div>{\nconsole.log(hey)\n}</div>';
	const output = await testTSXSourcemap(input, 'log');
	assert.equal(output, {
		source: 'index.astro',
		line: 2,
		column: 9,
		name: null,
	});
});

test('template expression has dot', async () => {
	const input = '<div>{\r\nconsole.log(hey)\r\n}</div>';
	const output = await testTSXSourcemap(input, 'log');
	assert.equal(output, {
		source: 'index.astro',
		line: 2,
		column: 9,
		name: null,
	});
});

test('template expression with addition', async () => {
	const input = `{"hello" + \nhey}`;
	const output = await testTSXSourcemap(input, 'hey');
	assert.equal(output, {
		source: 'index.astro',
		line: 2,
		column: 1,
		name: null,
	});
});

test('template expression with addition', async () => {
	const input = `{"hello" + \r\nhey}`;
	const output = await testTSXSourcemap(input, 'hey');
	assert.equal(output, {
		source: 'index.astro',
		line: 2,
		column: 1,
		name: null,
	});
});

test('html attribute', async () => {
	const input = `<svg\nvalue="foo" color="#000"></svg>`;
	const output = await testTSXSourcemap(input, 'color');
	assert.equal(output, {
		source: 'index.astro',
		name: null,
		line: 2,
		column: 12,
	});
});

test('html attribute', async () => {
	const input = `<svg\r\nvalue="foo" color="#000"></svg>`;
	const output = await testTSXSourcemap(input, 'color');
	assert.equal(output, {
		source: 'index.astro',
		name: null,
		line: 2,
		column: 12,
	});
});

test('complex template expression', async () => {
	const input = `{[].map(ITEM => {\r\nv = "what";\r\nreturn <div>{ITEMS}</div>\r\n})}`;
	const item = await testTSXSourcemap(input, 'ITEM');
	const items = await testTSXSourcemap(input, 'ITEMS');
	assert.equal(item, {
		source: 'index.astro',
		name: null,
		line: 1,
		column: 8,
	});
	assert.equal(items, {
		source: 'index.astro',
		name: null,
		line: 3,
		column: 14,
	});
});

test('attributes', async () => {
	const input = `<div\r\na="b" className="hello" />`;
	const className = await testTSXSourcemap(input, 'className');
	assert.equal(className, {
		source: 'index.astro',
		name: null,
		line: 2,
		column: 6,
	});
});

test('special attributes', async () => {
	const input = `<div\r\na="b" @on.click="fn" />`;
	const onClick = await testTSXSourcemap(input, '@on.click');
	assert.equal(onClick, {
		source: 'index.astro',
		name: null,
		line: 2,
		column: 6,
	});
});

test('whitespace', async () => {
	const input = `---\r\nimport A from "a";\r\n\timport B from "b";\r\n---\r\n`;
	const { code } = await convertToTSX(input, { sourcemap: 'both', filename: 'index.astro' });
	assert.match(code, '\t', 'output includes \\t');

	const B = await testTSXSourcemap(input, 'B');
	assert.equal(B, {
		source: 'index.astro',
		name: null,
		line: 3,
		column: 9,
	});
});

test.run();
