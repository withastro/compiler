import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { convertToTSX } from '@astrojs/compiler';

test('handles plain aliases', async () => {
  const input = `---
interface LocalImageProps {}
type Props = LocalImageProps;
---`;
  const output = await convertToTSX(input, { filename: 'index.astro', sourcemap: 'inline' });
  assert.ok(output.code.includes('(_props: Props)'), 'Includes aliased Props as correct props');
});


test('handles aliases with nested generics', async () => {
  const input = `---
interface LocalImageProps {
  src: Promise<{ default: string }>;
}

type Props = LocalImageProps;
---`;
  const output = await convertToTSX(input, { filename: 'index.astro', sourcemap: 'inline' });
  assert.ok(output.code.includes('(_props: Props)'), 'Includes aliased Props as correct props');
});
