import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { TSXPrefix } from '../utils.js';

test('script function', async () => {
	const input = `<script type="module">console.log({ test: \`literal\` })</script>`;
	const output = `${TSXPrefix}<Fragment>
<script type="module">
{() => {console.log({ test: \`literal\` })}}
</script>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test('partytown function', async () => {
	const input = `<script type="text/partytown">console.log({ test: \`literal\` })</script>`;
	const output = `${TSXPrefix}<Fragment>
<script type="text/partytown">
{() => {console.log({ test: \`literal\` })}}
</script>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test('ld+json wrapping', async () => {
	const input = `<script type="application/ld+json">{"a":"b"}</script>`;
	const output = `${TSXPrefix}<Fragment>
<script type="application/ld+json">{\`{"a":"b"}\`}</script>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test('escape unknown types', async () => {
	const input = `<script type="text/somethigndf" is:inline>console.log("something");</script>`;
	const output = `${TSXPrefix}<Fragment>
<script type="text/somethigndf" is:inline>{\`console.log("something");\`}</script>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test("don't include scripts if disabled", async () => {
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
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test.run();
