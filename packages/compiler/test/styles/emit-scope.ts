import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
---
let value = 'world';
---

<style>div { color: red; }</style>

<div>Hello world!</div>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    sourcemap: true,
  });
});

test('emits a scope', () => {
  assert.ok(result.scope, 'Expected to return a scope');
});

test.run();
