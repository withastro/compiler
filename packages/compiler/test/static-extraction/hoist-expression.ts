import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `
---
const url = 'foo';
---
<script type="module" hoist src={url}></script>
`;

let result: TransformResult;
test.before(async () => {
	result = await transform(FIXTURE);
});

test('logs warning with hoisted expression', () => {
	assert.ok(result.code);
});

test.run();
