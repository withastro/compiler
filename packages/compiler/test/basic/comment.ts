import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `---
/***/
---

<div />
`;

let result: TransformResult;
test.before(async () => {
	result = await transform(FIXTURE);
});

test('Can handle multi-* comments', () => {
	assert.ok(result.code, 'Expected to compile');
	assert.equal(result.diagnostics.length, 0, 'Expected no diagnostics');
});

test.run();
