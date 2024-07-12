import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testTSXSourcemap } from '../utils';

test('script is:inline', async () => {
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

	assert.equal(output, {
		line: 8,
		column: 23,
		source: 'index.astro',
		name: null,
	});
});

test.run();
