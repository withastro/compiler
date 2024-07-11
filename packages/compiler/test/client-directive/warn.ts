import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `
<script client:load></script>
`;

let result: unknown;
test.before(async () => {
	result = await transform(FIXTURE);
});

test('reports a warning for using a client directive', () => {
	assert.ok(Array.isArray(result.diagnostics));
	assert.is(result.diagnostics.length, 2);
	assert.equal(result.diagnostics[0].severity, 2);
	assert.match(result.diagnostics[0].text, 'does not need the client:load directive');
});

test.run();
