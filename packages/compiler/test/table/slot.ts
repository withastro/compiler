import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';
import { parse } from 'acorn';

test('does not panic on table inside slot', async () => {
  const input = `---
import Main from '../layouts/main.astro'

const fileList = [
  { title: "a", files: ["a.pdf"] },
  { title: "b", files: ["b.pdf"] }
]
---

<Main>
  {
    fileList.map(list => (
      <table>
        <tbody>
          {list.files.map(file => (
            <tr>
              <td>{file}</td>
            </tr>
          ))}
        </tbody>
      </table>
    ))
  }
  <hr />
</Main>
`;

  let error = 0;
  try {
    const { code } = await transform(input, { sourcefile: 'index.astro', sourcemap: 'inline' });
    parse(code, { ecmaVersion: 'latest', sourceType: 'module' });
  } catch (e) {
    error = 1;
  }
  assert.equal(error, 0, `compiler should generate valid code`);
});
