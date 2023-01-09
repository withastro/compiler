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
    sourcefile: '/src/components/EOF.astro',
  });
});

test('html comment error', () => {
  assert.ok(Array.isArray(result.diagnostics));
  assert.is(result.diagnostics.length, 1);
  assert.is(result.diagnostics[0].text, 'Unterminated comment');
  assert.is(FIXTURE.split('\n')[result.diagnostics[0].location.line - 1], `      <!--`);
});

test.run();
