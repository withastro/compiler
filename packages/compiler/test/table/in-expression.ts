import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';
import { parse } from 'acorn';

test('does not panic on table in expression', async () => {
  const input = `
<section>
    {course.reviews && course.reviews.length &&
        <>
            <div class="py-3">
                <hr>
            </div>
            <h2 class="text-lg font-bold">รีวิวจากผู้เรียน (ทั้งหมด {course.reviews.length} รีวิว คะแนนเฉลี่ย {course.reviews.reduce((p, c, _, { length }) => p + c.star / length, 0).toFixed(1)})</h2>
            <table class="rounded shadow dark:shadow-none dark:border dark:border-gray-700">
                <tbody>
                {course.reviews.map(review => (
                    <tr class="even:bg-gray-50 dark:even:bg-gray-700">
                    <td class="p-2 align-top"><Icon class="w-8 h-8 flex-shrink-0" name="mdi:account-circle"></Icon></td>
                    <td class="p-2 w-full">
                        <h3 class="whitespace-nowrap font-bold">{review.name}</h3>
                        {review.comment && <p class="text-sm text-secondary">{review.comment}</p>}
                    </td>
                    <td class="p-2 align-top">{'⭐'.repeat(review.star)}</td>
                    </tr>
                ))}
                </tbody>
            </table>
        </>
    }
</section>
`;

  let error = 0;
  try {
    const { code } = await transform(input, { filename: 'index.astro', sourcemap: 'inline' });
    parse(code, { ecmaVersion: 'latest', sourceType: 'module' });
  } catch (e) {
    error = 1;
  }
  assert.equal(error, 0, `compiler should generate valid code`);
});

test('does not generate invalid markup on table in expression', async () => {
  const input = `
<ul>
    {Astro.props.page.data.map(page => 
        <li>
            <table>
            <tr><td>{page.frontmatter.title}</td></tr>
            <tr><td>
                <Debug {...Object.keys(page)} />
            </td></tr>
            </table>
        </li>
  )}
</ul>
`;

  let error = 0;
  try {
    const { code } = await transform(input, { filename: 'index.astro', sourcemap: 'inline' });
    parse(code, { ecmaVersion: 'latest', sourceType: 'module' });
  } catch (e) {
    error = 1;
  }
  assert.equal(error, 0, `compiler should generate valid code`);
});

test('does not generate invalid markup on multiple tables', async () => {
  const input = `
<section>
  {["a", "b", "c"].map(char=> {
    <table>
      <tbody>
        {[1, 2, 3].map((num) => (
          <tr>{num}</tr>
         ))}
      </tbody>
    </table>
})}
</section>
<section></section>
`;

  let error = 0;
  try {
    const { code } = await transform(input, { filename: 'index.astro', sourcemap: 'inline' });
    parse(code, { ecmaVersion: 'latest', sourceType: 'module' });
  } catch (e) {
    error = 1;
  }
  assert.equal(error, 0, `compiler should generate valid code`);
});
