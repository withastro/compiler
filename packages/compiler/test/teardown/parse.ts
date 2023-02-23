import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { parse, teardown } from '@astrojs/compiler';

const FIXTURE = `<div>hello</div>`;

test('parse still works after teardown', async () => {
  const ast1 = await parse(FIXTURE);
  assert.ok(ast1);
  teardown();
  const ast2 = await parse(FIXTURE);
  assert.ok(ast2);
});

test.run();
