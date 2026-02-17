import { transform, preprocessStyles } from '@astrojs/compiler-rs';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

const FIXTURE = `
<style lang="scss">
	article:global(:is(h1, h2, h3, h4, h5, h6):hover {
		color: purple;
	}
</style>
<style lang="scss">
	article:is(h1, h2, h3, h4, h5, h6)):hover {
		color: purple;
	}
</style>
`;

describe('bad-styles/sass', () => {
	it('it works', async () => {
		const preprocessedStyles = await preprocessStyles(FIXTURE, async () => {
			return {
				error: new Error('Unable to convert').message,
			};
		});
		const result = transform(FIXTURE, {
			filename: '/users/astro/apps/pacman/src/pages/index.astro',
			preprocessedStyles,
		});
		assert.deepStrictEqual(result.styleError.length, 2);
		assert.deepStrictEqual(result.styleError[0], 'Unable to convert');
	});
});
