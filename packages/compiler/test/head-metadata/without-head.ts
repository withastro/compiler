import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
<slot />
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    filename: 'test.astro',
  });
});

test('containsHead is false', () => {
  assert.equal(result.containsHead, false);
});

test.run();
