import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

test('reports a hint for adding attributes to a script tag without is:inline', async () => {
	const result = await transform(`<script type="module"></script>`);
	assert.equal(result.diagnostics[0].severity, 4);
	assert.match(result.diagnostics[0].text, /\#script-processing/);
});

test('does not report a diagnostic for the src attribute', async () => {
	const result = await transform(`<script src="/external.js"></script>`);
	console.log(result.diagnostics);
	assert.equal(result.diagnostics.length, 0);
});

test.run();
