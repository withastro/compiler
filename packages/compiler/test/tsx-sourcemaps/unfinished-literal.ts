import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { convertToTSX } from '@astrojs/compiler';

const input = `<div class=\`></div>
`;

test('does not panic on unfinished template literal attribute', async () => {
  let error = 0;
  try {
    const output = await convertToTSX(input, { sourcefile: 'index.astro', sourcemap: 'inline' });
    assert.match(output.code, `class={\`\`}`);
  } catch (e) {
    error = 1;
  }

  assert.equal(error, 0, `compiler should not have panicked`);
});
