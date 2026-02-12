import { convertToTSX } from '@astrojs/compiler';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { TSXPrefix } from '../../utils.js';

describe('tsx/comment-whitespace', { skip: true }, () => {
	it('preverve whitespace around jsx comments', async () => {
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
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});
});
