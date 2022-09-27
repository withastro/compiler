import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { convertToTSX } from '@astrojs/compiler';

test('style is raw', async () => {
  const input = `<style>div { color: red; }</style>`;
  const output = `<style>{\`div { color: red; }\`}</style>

export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('is:raw is raw', async () => {
  const input = `<div is:raw>A{B}C</div>`;
  const output = `<div is:raw>{\`A{B}C\`}</div>

export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test.run();
