import { transform, preprocessStyles } from '@astrojs/compiler';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { preprocessStyle } from '../utils.js';

describe('styles/define-vars', () => {
	it('does not include define:vars in generated markup', async () => {
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
		const preprocessedStyles = await preprocessStyles(input, preprocessStyle);
		const result = transform(input, {
			preprocessedStyles,
		});
		assert.ok(!result.code.includes('STYLES'));
		assert.deepStrictEqual(result.css.length, 1);
	});

	it('handles style object and define:vars', () => {
		const input = `
---
let color = 'red';
---

<div style={{ color: 'var(--color)' }}>Hello world!</div>

<style define:vars={{ color }}></style>
`;
		const result = transform(input);
		// Oxc codegen uses double quotes and may add spaces after commas
		assert.ok(
			result.code.includes('$$addAttribute([{ color: "var(--color)" }, $$definedVars], "style")') ||
				result.code.includes(`$$addAttribute([{ color: 'var(--color)' },$$definedVars], "style")`),
		);
	});
});
