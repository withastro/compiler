import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { convertToTSX } from '@astrojs/compiler';

test('does not panic on unfinished template literal attribute', async () => {
  const input = `<div class=\`></div>
  `;
  let error = 0;
  try {
    const output = await convertToTSX(input, { filename: 'index.astro', sourcemap: 'inline' });
    assert.match(output.code, `class={\`\`}`);
  } catch (e) {
    error = 1;
  }

  assert.equal(error, 0, `compiler should not have panicked`);
});

test('does not panic on unfinished double quoted attribute', async () => {
  const input = `<main id="gotcha />`;
  let error = 0;
  try {
    const output = await convertToTSX(input, { filename: 'index.astro', sourcemap: 'inline' });
    assert.match(output.code, `id=""`);
  } catch (e) {
    error = 1;
  }

  assert.equal(error, 0, `compiler should not have panicked`);
});

test('does not panic on unfinished single quoted attribute', async () => {
  const input = `<main id='gotcha />`;
  let error = 0;
  try {
    const output = await convertToTSX(input, { filename: 'index.astro', sourcemap: 'inline' });
    assert.match(output.code, `id=""`);
  } catch (e) {
    console.log;
    error = 1;
  }

  assert.equal(error, 0, `compiler should not have panicked`);
});
