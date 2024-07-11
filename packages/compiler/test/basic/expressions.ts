import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

test('Can handle < inside JSX expression', async () => {
	const input = `<Layout>
   {
      new Array(totalPages).fill(0).map((_, index) => {
        const active = currentPage === index;
        if (
          totalPages > 25 &&
          ( index < currentPage - offset ||
            index > currentPage + offset)
        ) {
          return 'HAAAA';
        }
      })
    }
</Layout>
`;
	const output = await transform(input);
	assert.ok(output.code, 'Expected to compile');
	assert.match(
		output.code,
		`new Array(totalPages).fill(0).map((_, index) => {
        const active = currentPage === index;
        if (
          totalPages > 25 &&
          ( index < currentPage - offset ||
            index > currentPage + offset)
        ) {
          return 'HAAAA';
        }
      })`,
		'Expected expression to be compiled properly'
	);
	assert.equal(output.diagnostics.length, 0, 'Expected no diagnostics');
});

test.run();
