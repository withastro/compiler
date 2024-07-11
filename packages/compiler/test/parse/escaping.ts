import { parse } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const STYLE = 'div { & span { color: red; }}';
const FIXTURE = `<style>${STYLE}</style>`;

test('ampersand', async () => {
  const result = await parse(FIXTURE);
  assert.ok(result.ast, 'Expected an AST to be generated');
  const [
    {
      children: [{ value: output }],
    },
  ] = result.ast.children;
  assert.equal(output, STYLE, 'Expected AST style to equal input');
});

test.run();
