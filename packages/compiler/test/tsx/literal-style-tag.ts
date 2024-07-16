import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { TSXPrefix } from '../utils';

test('preserve style tag position I', async () => {
	const input = `<html><body><h1>Hello world!</h1></body></html>
<style></style>`;
	const output = `${TSXPrefix}<Fragment>
<html><body><h1>Hello world!</h1></body></html>
<style></style>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test('preserve style tag position II', async () => {
	const input = `<html></html>
<style></style>`;
	const output = `${TSXPrefix}<Fragment>
<html></html>
<style></style>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test('preserve style tag position III', async () => {
	const input = `<html lang="en"><head><BaseHead /></head></html>
<style>@use "../styles/global.scss";</style>`;
	const output = `${TSXPrefix}<Fragment>
<html lang="en"><head><BaseHead /></head></html>
<style>{\`@use "../styles/global.scss";\`}</style>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test('preserve style tag position IV', async () => {
	const input = `<html lang="en"><head><BaseHead /></head><body><Header /></body></html>
<style>@use "../styles/global.scss";</style>`;
	const output = `${TSXPrefix}<Fragment>
<html lang="en"><head><BaseHead /></head><body><Header /></body></html>
<style>{\`@use "../styles/global.scss";\`}</style>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test('preserve style tag position V', async () => {
	const input = `<html lang="en"><head><BaseHead /></head><body><Header /></body><style>@use "../styles/global.scss";</style></html>`;
	const output = `${TSXPrefix}<Fragment>
<html lang="en"><head><BaseHead /></head><body><Header /></body><style>{\`@use "../styles/global.scss";\`}</style></html>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.snapshot(code, output, 'expected code to match snapshot');
});

test.run();
