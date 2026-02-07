import { convertToTSX } from '@astrojs/compiler';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { TSXPrefix } from '../utils.js';

describe('tsx/raw', { skip: true }, () => {
	it('style is raw', async () => {
		const input = '<style>div { color: red; }</style>';
		const output = `${TSXPrefix}<Fragment>
<style>{\`div { color: red; }\`}</style>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('is:raw is raw', async () => {
		const input = '<div is:raw>A{B}C</div>';
		const output = `${TSXPrefix}<Fragment>
<div is:raw>{\`A{B}C\`}</div>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});
});
