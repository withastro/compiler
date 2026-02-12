import { convertToTSX } from '@astrojs/compiler';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { TSXPrefix } from '../../utils.js';

const PREFIX = (component = '__AstroComponent_') => `/**
 * Astro global available in all contexts in .astro files
 *
 * [Astro documentation](https://docs.astro.build/reference/api-reference/#astro-global)
*/
declare const Astro: Readonly<import('astro').AstroGlobal<Props, typeof ${component}>>`;

describe('tsx/props', { skip: true }, () => {
	it('no props', async () => {
		const input = '<div></div>';
		const output = `${TSXPrefix}<Fragment>
<div></div>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('nested Props', async () => {
		const input = `---
function DoTheThing(Props) {}
---`;
		const output = `${TSXPrefix}
function DoTheThing(Props) {}


export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('props interface', async () => {
		const input = `
---
interface Props {}
---

<div></div>
`;
		const output = `${TSXPrefix}
interface Props {}

{};<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_(_props: Props): any {}
${PREFIX()}`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('props import', async () => {
		const input = `
---
import { Props } from './somewhere';
---

<div></div>
`;
		const output = `${TSXPrefix}
import { Props } from './somewhere';

<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_(_props: Props): any {}
${PREFIX()}`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('props alias', async () => {
		const input = `
---
import { MyComponent as Props } from './somewhere';
---

<div></div>
`;
		const output = `${TSXPrefix}
import { MyComponent as Props } from './somewhere';

<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_(_props: Props): any {}
${PREFIX()}`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('props type import', async () => {
		const input = `
---
import type { Props } from './somewhere';
---

<div></div>
`;
		const output = `${TSXPrefix}
import type { Props } from './somewhere';

<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_(_props: Props): any {}
${PREFIX()}`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('props type', async () => {
		const input = `
---
type Props = {}
---

<div></div>
`;
		const output = `${TSXPrefix}
type Props = {}

{};<Fragment>
<div></div>

</Fragment>
export default function Test__AstroComponent_(_props: Props): any {}
${PREFIX('Test__AstroComponent_')}`;
		const { code } = await convertToTSX(input, {
			filename: '/Users/nmoo/test.astro',
			sourcemap: 'external',
		});
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('props generic (simple)', async () => {
		const input = `
---
interface Props<T> {}
---

<div></div>
`;
		const output = `${TSXPrefix}
interface Props<T> {}

{};<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_<T>(_props: Props<T>): any {}
${PREFIX()}`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('props generic (complex)', async () => {
		const input = `
---
interface Props<T extends Other<{ [key: string]: any }>> {}
---

<div></div>
`;
		const output = `${TSXPrefix}
interface Props<T extends Other<{ [key: string]: any }>> {}

{};<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_<T extends Other<{ [key: string]: any }>>(_props: Props<T>): any {}
${PREFIX()}`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('props generic (very complex)', async () => {
		const input = `
---
interface Props<T extends { [key: string]: any }, P extends string ? { [key: string]: any }: never> {}
---

<div></div>
`;
		const output = `${TSXPrefix}
interface Props<T extends { [key: string]: any }, P extends string ? { [key: string]: any }: never> {}

{};<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_<T extends { [key: string]: any }, P extends string ? { [key: string]: any }: never>(_props: Props<T, P>): any {}
${PREFIX()}`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('props generic (very complex II)', async () => {
		const input = `
---
interface Props<T extends Something<false> ? A : B, P extends string ? { [key: string]: any }: never> {}
---

<div></div>
`;
		const output = `${TSXPrefix}
interface Props<T extends Something<false> ? A : B, P extends string ? { [key: string]: any }: never> {}

{};<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_<T extends Something<false> ? A : B, P extends string ? { [key: string]: any }: never>(_props: Props<T, P>): any {}
${PREFIX()}`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('polymorphic props', async () => {
		const input = `
---
interface Props<Tag extends keyof JSX.IntrinsicElements> extends HTMLAttributes<Tag> {
  as?: Tag;
}
---

<div></div>
`;
		const output = `${TSXPrefix}
interface Props<Tag extends keyof JSX.IntrinsicElements> extends HTMLAttributes<Tag> {
  as?: Tag;
}

{};<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_<Tag extends keyof JSX.IntrinsicElements>(_props: Props<Tag>): any {}
${PREFIX()}`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('unrelated prop import', async () => {
		const input = `
---
import SvelteOptionalProps from './SvelteOptionalProps.svelte';
---

<SvelteOptionalProps />
`;
		const output = `${TSXPrefix}
import SvelteOptionalProps from './SvelteOptionalProps.svelte';

<Fragment>
<SvelteOptionalProps />

</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('unrelated sibling prop', async () => {
		const input = `---
import type { Props as ComponentBProps } from './ComponentB.astro'
---

<div />
`;
		const output = `${TSXPrefix}
import type { Props as ComponentBProps } from './ComponentB.astro'

{};<Fragment>
<div />

</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});
});
