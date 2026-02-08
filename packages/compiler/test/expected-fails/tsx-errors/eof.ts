import { convertToTSX } from '@astrojs/compiler';
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';
import type { TSXResult } from '../../../types.js';

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

describe('tsx-errors/eof', { skip: true }, () => {
	let result: TSXResult;
	before(async () => {
		result = await convertToTSX(FIXTURE, {
			filename: '/src/components/EOF.astro',
		});
	});

	it('got a tokenizer error', () => {
		assert.ok(Array.isArray(result.diagnostics));
		assert.strictEqual(result.diagnostics.length, 1);
		assert.strictEqual(result.diagnostics[0].text, 'Unterminated comment');
		assert.strictEqual(FIXTURE.split('\n')[result.diagnostics[0].location.line - 1], '      {/*');
	});
});
