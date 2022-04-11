import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
---
const item = 'a';
---

<Switch on={item}>
  <Case is="a">Item is A</Case>
  <Default>Unknown</Default>
</Switch>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE);
});

test('can compile switch/case/default', () => {
  assert.ok(result.code, 'Expected to compiler body expression!');
});

test('correctly compiles switch/case/default', () => {
  assert.ok(result.code.includes('switch ((item))'));
});

test.run();
