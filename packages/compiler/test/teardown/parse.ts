import { parse, teardown } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = '<div>hello</div>';

test('parse still works after teardown', async () => {
  const ast1 = await parse(FIXTURE);
  assert.ok(ast1);
  teardown();
  // Make sure `parse` creates a new WASM instance after teardown removed the previous one
  const ast2 = await parse(FIXTURE);
  assert.ok(ast2);
});

test.run();
