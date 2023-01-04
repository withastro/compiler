import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
---
---

<style>
div {
  background-color: blue;
  width: 50px;
  height: 50px;
}
</style>

<div />
`.trim();

function grabAstroScope(code: string) {
  let match = /astro-[0-9A-Za-z]+/.exec(code);
  if (match) {
    return match[0];
  }
  return null;
}

test('Similar components have different scoped class names', async () => {
  let result = await transform(FIXTURE, {
    moduleId: '/src/pages/index.astro',
  });
  let scopeA = grabAstroScope(result.code);
  assert.ok(scopeA);

  result = await transform(FIXTURE, {
    moduleId: '/src/pages/two.astro',
  });

  let scopeB = grabAstroScope(result.code);
  assert.ok(scopeB);

  assert.ok(scopeA !== scopeB, 'The scopes should not match for different files');
});

test.run();
