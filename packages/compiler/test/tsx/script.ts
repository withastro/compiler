import { convertToTSX } from '@astrojs/compiler';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { TSXPrefix } from '../utils.js';

describe('tsx/script', { skip: true }, () => {
	it('script function', async () => {
		const input = `<script type="module">console.log({ test: \`literal\` })</script>`;
		const output = `${TSXPrefix}<Fragment>
<script type="module">
{() => {console.log({ test: \`literal\` })}}
</script>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('partytown function', async () => {
		const input = `<script type="text/partytown">console.log({ test: \`literal\` })</script>`;
		const output = `${TSXPrefix}<Fragment>
<script type="text/partytown">
{() => {console.log({ test: \`literal\` })}}
</script>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('ld+json wrapping', async () => {
		const input = `<script type="application/ld+json">{"a":"b"}</script>`;
		const output = `${TSXPrefix}<Fragment>
<script type="application/ld+json">{\`{"a":"b"}\`}</script>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('escape unknown types', async () => {
		const input = `<script type="text/somethigndf" is:inline>console.log("something");</script>`;
		const output = `${TSXPrefix}<Fragment>
<script type="text/somethigndf" is:inline>{\`console.log("something");\`}</script>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it("don't include scripts if disabled", async () => {
		const input = `
<script>hello;</script>
<script type="module">hello;</script>
<script type="text/partytown">hello;</script>
<script type="application/ld+json">hello;</script>
<script type="text/somethigndf" is:inline>hello;</script>`;
		const output = `${TSXPrefix}<Fragment>
<script></script>
<script type="module"></script>
<script type="text/partytown"></script>
<script type="application/ld+json"></script>
<script type="text/somethigndf" is:inline></script>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external', includeScripts: false });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});
});
