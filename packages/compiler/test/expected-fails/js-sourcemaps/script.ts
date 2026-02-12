import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testJSSourcemap } from '../../utils.js';

describe('js-sourcemaps/script', { skip: true }, () => {
	it('script is:inline', async () => {
		const input = `<script is:inline>
  const MyNumber = 3;
  console.log(MyNumber.toStrang());
</script>
`;
		const output = await testJSSourcemap(input, '\n');

		assert.deepStrictEqual(output, {
			line: 1,
			column: 18,
			source: 'index.astro',
			name: null,
		});
	});
});
