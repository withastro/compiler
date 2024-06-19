import { parse } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = '{foo}，';

test('does not crash', async () => {
  const result = await parse(FIXTURE);
  assert.ok(result.ast, 'does not crash');
});

test('properly maps the position', async () => {
  const {
    ast: { children },
  } = await parse(FIXTURE);
  const text = children[1];
  assert.equal(text.position.start.offset, 5, 'properly maps the text start position');
  assert.equal(text.position.end.offset, 8, 'properly maps the text end position');
});

test.run();
