import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
  <script data-astro-rerun>"Bar"</script>
  <script data-astro-rerun src="some.js" type="module" />
`;

test('warns about data-astro-rerun on external ESMs', async () => {
  const result = await transform(FIXTURE);
  assert.equal(result.diagnostics.length, 1);
  assert.equal(result.diagnostics[0].code, 2010);
});

test.run();
