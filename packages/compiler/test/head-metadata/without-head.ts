import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `
<slot />
`;

let result: unknown;
test.before(async () => {
  result = await transform(FIXTURE, {
    filename: 'test.astro',
  });
});

test('containsHead is false', () => {
  assert.equal(result.containsHead, false);
});

test.run();
