import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
<style lang="scss">
	article:global(:is(h1, h2, h3, h4, h5, h6):hover {
		color: purple;
	}
</style>
`;

test('it works', async () => {
  let result = await transform(FIXTURE, {
    experimentalStaticExtraction: true,
    pathname: '/@fs/users/astro/apps/pacman/src/pages/index.astro',
    async preprocessStyle() {
      return {
        error: new Error('Unable to convert').message,
      };
    },
  });
  assert.equal(result.styleError, 'Unable to convert');
});

test.run();
