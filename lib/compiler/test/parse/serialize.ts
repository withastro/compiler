import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { parse } from '@astrojs/compiler';
import { serialize } from '@astrojs/compiler/utils';

const FIXTURE = `---
let value = 'world';
---

<style>
  :root {
    color: red;
  }
</style>

<div>Hello {value}</div>

<Markdown>
  # Hello world!
</Markdown>
`;

let result;
test.before(async () => {
  const { ast } = await parse(FIXTURE);
  result = serialize(ast);
});

test('serialize', () => {
  assert.type(result, 'string', `Expected "serialize" to return an object!`);
  assert.equal(result, FIXTURE, `Expected serialized output to equal input`);
});

test.run();
