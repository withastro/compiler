import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { parse } from '@astrojs/compiler';

test('can compile unfinished style', async () => {
  let error = 0;
  try {
    await parse(`<style>`);
  } catch (e) {
    error = 1;
  }
  assert.equal(error, 0, 'Expected to compile with unfinished style.');
});

test.run();
