import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testTSXSourcemap } from '../../utils.js';

describe('tsx-sourcemaps/error', { skip: true }, () => {
	it('svelte error', async () => {
		const input = `---
import SvelteOptionalProps from "./SvelteOptionalProps.svelte"
---

<SvelteOptionalProps></SvelteOptionalProps>`;
		const output = await testTSXSourcemap(input, '<SvelteOptionalProps>');

		assert.deepStrictEqual(output, {
			line: 5,
			column: 1,
			source: 'index.astro',
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
		const svelte = await testTSXSourcemap(input, '<SvelteError>');

		assert.deepStrictEqual(svelte, {
			line: 6,
			column: 1,
			source: 'index.astro',
			name: null,
		});

		const vue = await testTSXSourcemap(input, '<VueError>');

		assert.deepStrictEqual(vue, {
			line: 7,
			column: 1,
			source: 'index.astro',
			name: null,
		});
	});
});
