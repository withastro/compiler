import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
<div xmlns:happy="https://example.com/schemas/happy">
  <img src="jolly.avif" happy:smile="sweet"/>
</div>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    filename: '/Users/matthew/dev/astro/packages/astro/test/fixtures/astro-attrs/src/pages/namespaced.astro',
    sourcemap: 'both',
  });
});

test('Includes null characters', () => {
  assert.not.match(result.code, '\x00', 'Corrupted output');
});

test.run();
