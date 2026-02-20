import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testJSSourcemap } from '../utils.js';

describe('js-sourcemaps/module', () => {
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
		const output = await testJSSourcemap(input, `'./script'`);

		assert.deepStrictEqual(output, {
			source: 'index.astro',
			line: 8,
			column: 2,
			name: null,
		});
	});
});
