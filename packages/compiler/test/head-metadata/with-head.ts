import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
<html>
  <head>
    <title>Testing</title>
  </head>
  <body>
    <h1>Testing</h1>
  </body>
</html>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    filename: 'test.astro',
  });
});

test('containsHead is true', () => {
  assert.equal(result.containsHead, true);
});

test.run();
