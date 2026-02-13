import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testJSSourcemap } from '../utils.js';

describe('js-sourcemaps/error', () => {
	it('svelte error', async () => {
		const input = `---
import SvelteOptionalProps from "./SvelteOptionalProps.svelte"
---

<SvelteOptionalProps></SvelteOptionalProps>`;
		const output = await testJSSourcemap(input, '<SvelteOptionalProps>');

		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 5,
			column: 0,
			name: null,
		});
	});

	it('vue error', async () => {
		const input = `---
import SvelteError from "./SvelteError.svelte"
import VueError from "./VueError.vue"
---

<SvelteError></SvelteError>
<VueError></VueError>`;
		const svelte = await testJSSourcemap(input, '<SvelteError>');

		assert.deepStrictEqual(svelte, {
			source: 'index.astro',
			line: 6,
			column: 0,
			name: null,
		});

		const vue = await testJSSourcemap(input, '<VueError>');

		assert.deepStrictEqual(vue, {
			source: 'index.astro',
			line: 7,
			column: 0,
			name: null,
		});
	});
});
