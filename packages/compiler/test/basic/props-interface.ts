import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `---
// eslint-disable-next-line no-undef
export interface Props extends astroHTML.JSX.HTMLAttributes {}

const props = { ...Astro.props } as Props;
---

<body class:list={props['class:list']}>
  <slot></slot>
</body>`;

let result: TransformResult;
test.before(async () => {
	result = await transform(FIXTURE);
});

test('retains newlines around comment', () => {
	assert.ok(result.code, 'Expected to compile');
	assert.match(result.code, /\/\/ eslint-disable-next-line no-undef\n/g);
	assert.equal(result.diagnostics.length, 0, 'Expected no diagnostics');
});

test.run();
