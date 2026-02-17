import { convertToTSX } from '@astrojs/compiler-rs';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { TSXPrefix } from '../../utils.js';

describe('tsx/escape', { skip: true }, () => {
	it('escapes braces in comment', async () => {
		const input = '<!-- {<div>Not JSX!<div/>}-->';
		const output = `${TSXPrefix}<Fragment>
{/** \\\\{<div>Not JSX!<div/>\\\\}*/}
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('always inserts space before comment', async () => {
		const input = '<!--/<div>Error?<div/>-->';
		const output = `${TSXPrefix}<Fragment>
{/** /<div>Error?<div/>*/}
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('simple escapes star slashes (*/)', async () => {
		const input = '<!--*/<div>Evil comment<div/>-->';
		const output = `${TSXPrefix}<Fragment>
{/** *\\/<div>Evil comment<div/>*/}
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('multiple escapes star slashes (*/)', async () => {
		const input = '<!--***/*/**/*/*/*/<div>Even more evil comment<div/>-->';
		const output = `${TSXPrefix}<Fragment>
{/** ***\\/*\\/**\\/*\\/*\\/*\\/<div>Even more evil comment<div/>*/}
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('does not escape tag opening unnecessarily', async () => {
		const input = `<div></div>
<div`;
		const output = `${TSXPrefix}<Fragment>
<div></div>
<div
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('does not escape tag opening unnecessarily II', async () => {
		const input = `<div>
<div
</div>
`;
		const output = `${TSXPrefix}<Fragment>
<div>
<div div {...{"<":true}}>
</div></div>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('does not escape tag opening unnecessarily III', async () => {
		const input = '<div>{[].map((something) => <div><Blocknote</div><div><Image</div>)}</div>';
		const output = `${TSXPrefix}<Fragment>
<div>{[].map((something) => <Fragment><div><Blocknote< div><div><Image< div>)</Image<></div></Blocknote<></div></Fragment>}</div>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});
});
