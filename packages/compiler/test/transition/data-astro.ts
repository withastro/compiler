import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
<div data-astro-reload>
  <a href="/" data-astro-reload>/</a>
  <form data-astro-reload="x">.</form>
  <area data-astro-reload/>
  <svg xmlns="http://www.w3.org/2000/svg"><a data-astro-reload>.</a></svg>
  <script is:inline data-astro-rerun src="some.js" type="module" />
  <script is:inline data-astro-rerun>"Bar"</script>
</div>`;

test('Issues warnings for data-astro-* attributes', async () => {
  const result = await transform(FIXTURE);
  assert.equal(result.diagnostics.length, 3);
  assert.equal(result.diagnostics[0].code, 2000);
  assert.equal(result.diagnostics[1].code, 2005);
  assert.equal(result.diagnostics[2].code, 2010);
});

test.run();
