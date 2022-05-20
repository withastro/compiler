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

let error: Error;
test.before(async () => {
  try {
    await transform(FIXTURE, {
      pathname: '/src/components/Cool.astro',
    });
  } catch (err) {
    error = err;
  }
});

test('got an error because client:only component not found import', () => {
  assert.ok(error, 'paniced');
});

/*
test('exports named component', () => {
  assert.match(result.code, 'export default $$Cool', 'Expected output to contain named export');
});
*/

test.run();
