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

test('include start and end positions for comments', async () => {
  const input = `---
// Hello world!
---

<!-- prettier:ignore -->
<iframe>Hello</iframe><div></div>`;
  const { ast } = await parse(input);

  const comment = ast.children[1];
  assert.is(comment.type, 'comment');
  assert.ok(comment.position.start, `Expected serialized output to contain a start position`);
  assert.ok(comment.position.end, `Expected serialized output to contain an end position`);
});

test('include start and end positions for text', async () => {
  const input = `---
// Hello world!
---

Hello world!`;
  const { ast } = await parse(input);

  const text = ast.children[1];
  assert.is(text.type, 'text');
  assert.ok(text.position.start, `Expected serialized output to contain a start position`);
  assert.ok(text.position.end, `Expected serialized output to contain an end position`);
});

test('include start and end positions for self-closing tags', async () => {
  const input = `<input/>`;
  const { ast } = await parse(input);

  const element = ast.children[0];
  assert.is(element.type, 'element');
  assert.is(element.name, 'input');
  assert.ok(element.position.start, `Expected serialized output to contain a start position`);
  assert.ok(element.position.end, `Expected serialized output to contain an end position`);
});

test.run();
