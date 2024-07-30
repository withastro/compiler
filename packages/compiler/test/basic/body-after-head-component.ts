import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `---
const isProd = true;
---
<!DOCTYPE html>
<html lang="en">
  <head>
    {isProd && <TestHead />}
    <title>document</title>
    {isProd && <slot />}
  </head>
  <body style="color: red;">
    <main>
       <h1>Welcome to <span class="text-gradient">Astro</span></h1>
    </main>
  </body>
</html>
`;

let result: TransformResult;
test.before(async () => {
	result = await transform(FIXTURE);
});

test('has body in output', () => {
	assert.match(
		result.code,
		'<body style="color: red;">',
		'Expected output to contain body element!'
	);
});

test.run();
