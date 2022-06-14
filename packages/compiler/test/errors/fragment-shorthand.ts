
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    < data-test="hello"><div></div></>
  </body>
</html>`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    pathname: '/src/components/fragment.astro',
  });
});

test('got a tokenizer error', () => {
  console.log(result);
  assert.ok(Array.isArray(result.errors));
  assert.is(result.errors.length, 1);
  assert.is(result.errors[0].text, 'Unable to assign attributes when using <> Fragment shorthand syntax!');
  assert.is(FIXTURE.split('\n')[result.errors[0].location.line - 1], `    < data-test="hello"><div></div></>`);
});

test.run();
