import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `<div>Hello world!</div>`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    pathname: '/src/components/Cool.astro',
  });
});

test('exports named component', () => {
  assert.match(result.code, 'export default $$Cool', 'Expected output to contain named export');
});

test.run();
