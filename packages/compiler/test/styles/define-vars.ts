import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { preprocessStyle } from '../utils.js';

test('does not include define:vars in generated markup', async () => {
	const input = `
---
let color = 'red';
---

<style lang="scss" define:vars={{ color }}>
  div {
    color: var(--color);
  }
</style>

<div>Hello world!</div>

<div>Ahhh</div>
`;
	const result = await transform(input, {
		preprocessStyle,
	});
	assert.ok(!result.code.includes('STYLES'));
	assert.equal(result.css.length, 1);
});

test('handles style object and define:vars', async () => {
	const input = `
---
let color = 'red';
---

<div style={{ color: 'var(--color)' }}>Hello world!</div>

<style define:vars={{ color }}></style>
`;
	const result = await transform(input);
	assert.match(result.code, `$$addAttribute([{ color: 'var(--color)' },$$definedVars], "style")`);
});

test.run();
