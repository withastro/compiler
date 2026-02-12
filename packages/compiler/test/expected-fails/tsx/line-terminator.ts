import { convertToTSX } from '@astrojs/compiler';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

describe('tsx/line-terminator', { skip: true }, () => {
	it('handles non-standard line terminators', async () => {
		const inputs = [' ', 'something something', 'something  ', '   '];
		let err = 0;
		for (const input of inputs) {
			try {
				await convertToTSX(input, { filename: 'index.astro', sourcemap: 'inline' });
			} catch (_e) {
				err = 1;
			}
		}
		assert.deepStrictEqual(err, 0, 'did not error');
	});
});
