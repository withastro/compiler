import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
<div transition:reload>
  <script transition:reload="quick">"hu"</script>
</div>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    resolvePath: async (s) => s,
  });
});

test('tagged with propagation metadata', () => {
  assert.equal(result.diagnostics[0].code, 2010);
  assert.equal(result.diagnostics[1].code, 2005);
});

test.run();
