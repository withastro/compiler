import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `---
import * as components from '../components';
const { MyComponent } = components;
---
<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    <MyComponent client:only />
  </body>
</html>`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    filename: '/src/components/Cool.astro',
  });
});

test('got an error because client:only component not found import', () => {
  assert.ok(Array.isArray(result.diagnostics));
  assert.is(result.diagnostics.length, 1);
  assert.is(result.diagnostics[0].text, 'Unable to find matching import statement for client:only component');
  assert.is(FIXTURE.split('\n')[result.diagnostics[0].location.line - 1], `    <MyComponent client:only />`);
});

test.run();
