import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { convertToTSX } from '@astrojs/compiler';

const PREFIX = `/**
 * Astro global available in all contexts in .astro files
 *
 * [Astro documentation](https://docs.astro.build/reference/api-reference/#astro-global)
*/
declare const Astro: Readonly<import('astro').AstroGlobal<Props>>`;

test('no props', async () => {
  const input = `<div></div>`;
  const output = `<Fragment>
<div></div>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('nested Props', async () => {
  const input = `---
function DoTheThing(Props) {}
---`;
  const output = `
function DoTheThing(Props) {}


export default function __AstroComponent_(_props: Record<string, any>): any {}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('props interface', async () => {
  const input = `
---
interface Props {}
---

<div></div>
`;
  const output = `${PREFIX}
interface Props {}

"";<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_(_props: Props): any {}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('props import', async () => {
  const input = `
---
import { Props } from './somewhere';
---

<div></div>
`;
  const output = `${PREFIX}
import { Props } from './somewhere';

<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_(_props: Props): any {}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('props alias', async () => {
  const input = `
---
import { MyComponent as Props } from './somewhere';
---

<div></div>
`;
  const output = `${PREFIX}
import { MyComponent as Props } from './somewhere';

<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_(_props: Props): any {}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('props type import', async () => {
  const input = `
---
import type { Props } from './somewhere';
---

<div></div>
`;
  const output = `${PREFIX}
import type { Props } from './somewhere';

<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_(_props: Props): any {}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('props type', async () => {
  const input = `
---
type Props = {}
---

<div></div>
`;
  const output = `${PREFIX}
type Props = {}

"";<Fragment>
<div></div>

</Fragment>
export default function Test__AstroComponent_(_props: Props): any {}`;
  const { code } = await convertToTSX(input, { sourcefile: '/Users/nmoo/test.astro', sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('props generic (simple)', async () => {
  const input = `
---
interface Props<T> {}
---

<div></div>
`;
  const output = `${PREFIX}
interface Props<T> {}

"";<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_<T>(_props: Props<T>): any {}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('props generic (complex)', async () => {
  const input = `
---
interface Props<T extends Other<{ [key: string]: any }>> {}
---

<div></div>
`;
  const output = `${PREFIX}
interface Props<T extends Other<{ [key: string]: any }>> {}

"";<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_<T extends Other<{ [key: string]: any }>>(_props: Props<T>): any {}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('props generic (very complex)', async () => {
  const input = `
---
interface Props<T extends { [key: string]: any }, P extends string ? { [key: string]: any }: never> {}
---

<div></div>
`;
  const output = `${PREFIX}
interface Props<T extends { [key: string]: any }, P extends string ? { [key: string]: any }: never> {}

"";<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_<T extends { [key: string]: any }, P extends string ? { [key: string]: any }: never>(_props: Props<T, P>): any {}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('props generic (very complex II)', async () => {
  const input = `
---
interface Props<T extends Something<false> ? A : B, P extends string ? { [key: string]: any }: never> {}
---

<div></div>
`;
  const output = `${PREFIX}
interface Props<T extends Something<false> ? A : B, P extends string ? { [key: string]: any }: never> {}

"";<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_<T extends Something<false> ? A : B, P extends string ? { [key: string]: any }: never>(_props: Props<T, P>): any {}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('polymorphic props', async () => {
  const input = `
---
interface Props<Tag extends keyof JSX.IntrinsicElements> extends HTMLAttributes<Tag> {
  as?: Tag;
}
---

<div></div>
`;
  const output = `${PREFIX}
interface Props<Tag extends keyof JSX.IntrinsicElements> extends HTMLAttributes<Tag> {
  as?: Tag;
}

"";<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_<Tag extends keyof JSX.IntrinsicElements>(_props: Props<Tag>): any {}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('unrelated prop import', async () => {
  const input = `
---
import SvelteOptionalProps from './SvelteOptionalProps.svelte';
---

<SvelteOptionalProps />
`;
  const output = `
import SvelteOptionalProps from './SvelteOptionalProps.svelte';

<Fragment>
<SvelteOptionalProps />

</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test.run();
