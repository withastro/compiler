import { type TransformResult, transform } from '@astrojs/compiler';
import { before, describe, it } from 'node:test';
import assert from 'node:assert/strict';

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

describe('head-metadata/with-head', () => {
	let result: TransformResult;
	before(async () => {
		result = await transform(FIXTURE, {
			filename: 'test.astro',
		});
	});

	it('containsHead is true', () => {
		assert.deepStrictEqual(result.containsHead, true);
	});
});
