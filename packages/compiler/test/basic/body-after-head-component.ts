import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <TestHead />
    <title>document</title>
  </head>
  <body>
    <main>
       <h1>Welcome to <span class="text-gradient">Astro</span></h1>
    </main>
  </body>
</html>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE);
});

test('has body in output', () => {
  console.log(result.code);
  assert.match(result.code, '<body>', 'Expected output to contain body element!');
});

test.run();
