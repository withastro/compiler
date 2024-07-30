import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testJSSourcemap } from '../utils.js';

test('svelte error', async () => {
	const input = `---
import SvelteOptionalProps from "./SvelteOptionalProps.svelte"
---

<SvelteOptionalProps></SvelteOptionalProps>`;
	const output = await testJSSourcemap(input, '<SvelteOptionalProps>');

	assert.equal(output, {
		line: 5,
		column: 1,
		source: 'index.astro',
		name: null,
	});
});

test('vue error', async () => {
	const input = `---
import SvelteError from "./SvelteError.svelte"
import VueError from "./VueError.vue"
---

<SvelteError></SvelteError>
<VueError></VueError>`;
	const svelte = await testJSSourcemap(input, '<SvelteError>');

	assert.equal(svelte, {
		line: 6,
		column: 1,
		source: 'index.astro',
		name: null,
	});

	const vue = await testJSSourcemap(input, '<VueError>');

	assert.equal(vue, {
		line: 7,
		column: 1,
		source: 'index.astro',
		name: null,
	});
});

test.run();
