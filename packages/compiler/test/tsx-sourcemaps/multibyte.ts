import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testTSXSourcemap } from '../utils.js';

test('multibyte content', async () => {
	const input = '<h1>ツ</h1>';

	const output = await testTSXSourcemap(input, 'ツ');
	assert.equal(output, {
		source: 'index.astro',
		line: 1,
		column: 4,
		name: null,
	});
});

test('content after multibyte character', async () => {
	const input = '<h1>ツ</h1><p>foobar</p>';

	const output = await testTSXSourcemap(input, 'foobar');
	assert.equal(output, {
		source: 'index.astro',
		line: 1,
		column: 13,
		name: null,
	});
});

test('many characters', async () => {
	const input = '<h1>こんにちは</h1>';

	const output = await testTSXSourcemap(input, 'ん');
	assert.equal(output, {
		source: 'index.astro',
		line: 1,
		column: 5,
		name: null,
	});
});

test('many characters', async () => {
	const input = '<h1>こんにちは</h1>';

	const output = await testTSXSourcemap(input, 'に');
	assert.equal(output, {
		source: 'index.astro',
		line: 1,
		column: 6,
		name: null,
	});
});

test.run();
