import { type TransformResult, transform } from '@astrojs/compiler-rs';
import assert from 'node:assert/strict';
import { before, describe, it } from 'node:test';

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

describe('client-only-unfound', { skip: true }, () => {
	let result: TransformResult;
	before(async () => {
		result = await transform(FIXTURE, {
			filename: '/src/components/Cool.astro',
		});
	});

	it('got an error because client:only component not found import', () => {
		assert.ok(Array.isArray(result.diagnostics));
		assert.strictEqual(result.diagnostics.length, 1);
		assert.strictEqual(
			result.diagnostics[0].text,
			'Unable to find matching import statement for client:only component',
		);
		assert.strictEqual(
			FIXTURE.split('\n')[result.diagnostics[0].location.line - 1],
			'    <MyComponent client:only />',
		);
	});
});
