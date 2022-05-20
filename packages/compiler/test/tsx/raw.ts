import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { convertToTSX } from '@astrojs/compiler';

test('style is raw', async () => {
  const input = `<style>div { color: red; }</style>`;
  const output = `<Fragment>
<style>{\`div { color: red; }\`}</style>
</Fragment>

export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
  const { code } = await convertToTSX(input);
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('is:raw is raw', async () => {
  const input = `<div is:raw>A{B}C</div>`;
  const output = `<Fragment>
<div is:raw>{\`A{B}C\`}</div>
</Fragment>

export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
  const { code } = await convertToTSX(input);
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test.run();
