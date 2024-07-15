import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

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

let result: TransformResult;
test.before(async () => {
	result = await transform(FIXTURE, {
		filename: 'test.astro',
	});
});

test('containsHead is true', () => {
	assert.equal(result.containsHead, true);
});

test.run();
