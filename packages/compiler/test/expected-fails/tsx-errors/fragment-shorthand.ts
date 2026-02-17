import { convertToTSX } from '@astrojs/compiler-rs';
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';
import type { TSXResult } from '../../../types.js';

const FIXTURE = `<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    < data-test="hello"><div></div></>
  </body>
</html>`;

describe('tsx-errors/fragment-shorthand', { skip: true }, () => {
	let result: TSXResult;
	before(async () => {
		result = await convertToTSX(FIXTURE, {
			filename: '/src/components/fragment.astro',
		});
	});

	it('got a tokenizer error', () => {
		assert.ok(Array.isArray(result.diagnostics));
		assert.strictEqual(result.diagnostics.length, 1);
		assert.strictEqual(
			result.diagnostics[0].text,
			'Unable to assign attributes when using <> Fragment shorthand syntax!',
		);
		const loc = result.diagnostics[0].location;
		assert.strictEqual(FIXTURE.split('\n')[loc.line - 1], `    < data-test="hello"><div></div></>`);
		assert.strictEqual(
			FIXTURE.split('\n')[loc.line - 1].slice(loc.column - 1, loc.column - 1 + loc.length),
			`< data-test="hello">`,
		);
	});
});
