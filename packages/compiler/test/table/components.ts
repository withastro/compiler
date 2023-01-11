import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';
import { parse } from 'acorn';

test('allows components in table', async () => {
  const input = `
---
const  MyTableRow = "tr";
---

<table>
    <MyTableRow>
        <td>Witch</td>
    </MyTableRow>
    <MyTableRow>
        <td>Moon</td>
    </MyTableRow>
</table>
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
