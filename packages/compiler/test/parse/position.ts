import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { parse } from '@astrojs/compiler';

test('include start and end positions', async () => {
  const input = `---
// Hello world!
---

<iframe>Hello</iframe><div></div>`;
  const { ast } = await parse(input);
  assert.ok(ast.children.slice(1)[0].position.end, `Expected serialized output to contain an end position`);
});

test.run();
