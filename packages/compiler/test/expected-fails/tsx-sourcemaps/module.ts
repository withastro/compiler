import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testTSXSourcemap } from '../../utils.js';

describe('tsx-sourcemaps/module', { skip: true }, () => {
	it('script is:inline', async () => {
		const input = `---
  // valid
  import { foo } from './script.js';
    import ComponentAstro from './astro.astro';
    import ComponentSvelte from './svelte.svelte';
    import ComponentVue from './vue.vue';
  // invalid
  import { baz } from './script';
  foo;baz;ComponentAstro;ComponentSvelte;ComponentVue;
---
`;
		const output = await testTSXSourcemap(input, `'./script'`);

		assert.deepStrictEqual(output, {
			line: 8,
			column: 23,
			source: 'index.astro',
			name: null,
		});
	});
});
