import { type TransformResult, transform } from '@astrojs/compiler';
import assert from 'node:assert/strict';
import { before, describe, it } from 'node:test';

const FIXTURE = `<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    < data-test="hello"><div></div></>
  </body>
</html>`;

describe('fragment-shorthand', { skip: true }, () => {
	let result: TransformResult;
	before(async () => {
		result = await transform(FIXTURE, {
			filename: '/src/components/fragment.astro',
		});
	});

	it('got a tokenizer error', () => {
		assert.ok(Array.isArray(result.diagnostics));
		assert.strictEqual(result.diagnostics.length, 1);
		assert.strictEqual(
			result.diagnostics[0].text,
			'Unable to assign attributes when using <> Fragment shorthand syntax!'
		);
		const loc = result.diagnostics[0].location;
		assert.strictEqual(FIXTURE.split('\n')[loc.line - 1], `    < data-test="hello"><div></div></>`);
		assert.strictEqual(
			FIXTURE.split('\n')[loc.line - 1].slice(loc.column - 1, loc.column - 1 + loc.length),
			`< data-test="hello">`
		);
	});
});
