import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { convertToTSX } from '@astrojs/compiler';

test('script function', async () => {
  const input = `<script type="module">console.log({ test: \`literal\` })</script>`;
  const output = `<Fragment>
<script type="module">
{() => {console.log({ test: \`literal\` })}}
</script>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('partytown function', async () => {
  const input = `<script type="text/partytown">console.log({ test: \`literal\` })</script>`;
  const output = `<Fragment>
<script type="text/partytown">
{() => {console.log({ test: \`literal\` })}}
</script>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('ld+json wrapping', async () => {
  const input = `<script type="application/ld+json">{"a":"b"}</script>`;
  const output = `<Fragment>
<script type="application/ld+json">{\`{"a":"b"}\`}</script>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test.run();
