import { type TransformResult, transform } from '@astrojs/compiler-rs';
import assert from 'node:assert/strict';
import { before, describe, it } from 'node:test';

const FIXTURE = `<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    <div>
      {/*
    </div>
  </body>
</html>`;

describe('jsx-comment', { skip: true }, () => {
	let result: TransformResult;
	before(async () => {
		result = await transform(FIXTURE, {
			filename: '/src/components/EOF.astro',
		});
	});

	it('jsx comment error', () => {
		assert.ok(Array.isArray(result.diagnostics));
		assert.strictEqual(result.diagnostics.length, 1);
		assert.strictEqual(result.diagnostics[0].text, 'Unterminated comment');
		assert.strictEqual(FIXTURE.split('\n')[result.diagnostics[0].location.line - 1], '      {/*');
	});
});
