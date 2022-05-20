import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { parse } from '@astrojs/compiler';
import { serialize } from '@astrojs/compiler/utils';

const FIXTURE = `---
let value = 'world';
let content = "Testing 123";
---

<style>
  :root {
    color: red;
  }
</style>

<div>Hello {value}</div>

<h1 name="value" set:html={content} empty {shorthand} expression={true} literal=\`tags\`>Hello {value}</h1>

<Fragment set:html={content} />

<Markdown>
  # Hello world!
</Markdown>
`;

let result;
test.before(async () => {
  const { ast } = await parse(FIXTURE);
  try {
    result = serialize(ast);
  } catch (e) {
    // eslint-disable-next-line no-console
    console.log(e);
  }
});

test('serialize', () => {
  assert.type(result, 'string', `Expected "serialize" to return an object!`);
  assert.equal(result, FIXTURE, `Expected serialized output to equal input`);
});

test('self-close elements', async () => {
  const input = '<div />';
  const { ast } = await parse(input);
  const output = serialize(ast, { selfClose: false });
  const selfClosedOutput = serialize(ast);
  assert.equal(output, '<div></div>', `Expected serialized output to equal <div></div>`);
  assert.equal(selfClosedOutput, input, `Expected serialized output to equal ${input}`);
});

test.run();
