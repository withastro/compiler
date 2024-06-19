import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { TSXPrefix } from '../utils';

test('preverve whitespace around jsx comments', async () => {
  const input = `{/* @ts-expect-error */}
<Component prop="value"></Component>

{
// @ts-expect-error
}
<Component prop="value"></Component>

{
/* @ts-expect-error */
<Component prop="value"></Component>
}

{
// @ts-expect-error
<Component prop="value"></Component>
}`;
  const output = `${TSXPrefix}<Fragment>
{/* @ts-expect-error */}
<Component prop="value"></Component>

{
// @ts-expect-error
}
<Component prop="value"></Component>

{
/* @ts-expect-error */
<Fragment><Component prop="value"></Component></Fragment>
}

{
// @ts-expect-error
<Fragment><Component prop="value"></Component></Fragment>
}
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, 'expected code to match snapshot');
});

test.run();
