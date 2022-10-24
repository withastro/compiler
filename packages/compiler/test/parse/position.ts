import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { parse } from '@astrojs/compiler';

test('include start and end positions', async () => {
  const input = `---
// Hello world!
---

<iframe>Hello</iframe><div></div>`;
  const { ast } = await parse(input);

  const iframe = ast.children[1];
  assert.is(iframe.name, 'iframe');
  assert.ok(iframe.position.start, `Expected serialized output to contain a start position`);
  assert.ok(iframe.position.end, `Expected serialized output to contain an end position`);
});

test.run();
