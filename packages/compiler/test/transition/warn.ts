import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
<div transition:reload>
  <a href="." transition:reload>.</a>
  <form transition:reload>.</form>
  <area transition:reload />
  <svg xmlns="http://www.w3.org/2000/svg"><a transition:reload>.</a></svg>
  <script transition:reload="quick">"Foo"</script>
  <script transtision:reload>"Bar"</script>
  <script transition:reload src="some.js" type="module" />
</div>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    resolvePath: async (s) => s,
  });
});

test('has warnings', () => {
  assert.equal(result.diagnostics.length, 3);
  assert.equal(result.diagnostics[0].code, 2010);
  assert.equal(result.diagnostics[1].code, 2005);
  assert.equal(result.diagnostics[2].code, 2010);
});

test.run();
