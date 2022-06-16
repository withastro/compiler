
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    <div>
      <div slot="name" />
    </div>
  </body>
</html>`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    pathname: '/src/components/Slot.astro',
  });
});

test('got a slot error', () => {
  assert.ok(Array.isArray(result.errors));
  assert.is(result.errors.length, 1);
  assert.is(result.errors[0].text, "Element with a slot='...' attribute must be a child of a component or a descendant of a custom element");
  assert.is(FIXTURE.split('\n')[result.errors[0].location.line - 1], `      <div slot="name" />`);
});

test.run();
