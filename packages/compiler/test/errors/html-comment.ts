import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    <div>
      <!--
    </div>
  </body>
</html>`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    pathname: '/src/components/EOF.astro',
  });
});

test('html comment error', () => {
  assert.ok(Array.isArray(result.warnings));
  assert.is(result.warnings.length, 1);
  assert.is(result.warnings[0].text, 'Unterminated comment');
  assert.is(FIXTURE.split('\n')[result.warnings[0].location.line - 1], `      <!--`);
});

test.run();
