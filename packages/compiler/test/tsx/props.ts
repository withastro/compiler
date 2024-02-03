import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { TSXPrefix } from '../utils';

const PREFIX = (component: string = '__AstroComponent_') => `/**
 * Astro global available in all contexts in .astro files
 *
 * [Astro documentation](https://docs.astro.build/reference/api-reference/#astro-global)
*/
declare const Astro: Readonly<import('astro').AstroGlobal<Props, typeof ${component}>>`;

test('no props', async () => {
  const input = `<div></div>`;
  const output = `${TSXPrefix}<Fragment>
<div></div>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('nested Props', async () => {
  const input = `---
function DoTheThing(Props) {}
---`;
  const output = `${TSXPrefix}
function DoTheThing(Props) {}


export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
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
  const output = `${TSXPrefix}
interface Props {}

{};<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_(_props: Props): any {}
${PREFIX()}`;
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
  const output = `${TSXPrefix}
import { Props } from './somewhere';

<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_(_props: Props): any {}
${PREFIX()}`;
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
  const output = `${TSXPrefix}
import { MyComponent as Props } from './somewhere';

<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_(_props: Props): any {}
${PREFIX()}`;
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
  const output = `${TSXPrefix}
import type { Props } from './somewhere';

<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_(_props: Props): any {}
${PREFIX()}`;
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
  const output = `${TSXPrefix}
type Props = {}

{};<Fragment>
<div></div>

</Fragment>
export default function Test__AstroComponent_(_props: Props): any {}
${PREFIX('Test__AstroComponent_')}`;
  const { code } = await convertToTSX(input, { filename: '/Users/nmoo/test.astro', sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('props generic (simple)', async () => {
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
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('props generic (complex)', async () => {
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
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('props generic (very complex)', async () => {
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
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('props generic (very complex II)', async () => {
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
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('unrelated prop import', async () => {
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
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('unrelated sibling prop', async () => {
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
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('`as` in Props type field', async () => {
  const input = `---
interface Props {
  as?: string;
  href?: string;
}

const { as: Component, href } = Astro.props;
---

<Component {href} />
`;
  const output = `${TSXPrefix}
interface Props {
  as?: string;
  href?: string;
}

const { as: Component, href } = Astro.props;

<Fragment>
<Component href={href} />

</Fragment>
export default function __AstroComponent_(_props: Props): any {}
${PREFIX()}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

// TODO(mk): find a better place and better test name for this
test("type assertion doesn't prevent Props type detection", async () => {
  const input = `---
interface Props {
  myFunction: (value) => string;
}

const variable = { } as const;

Astro.props;
---`;
  const output = `${TSXPrefix}
interface Props {
  myFunction: (value) => string;
}

const variable = { } as const;

Astro.props;


export default function __AstroComponent_(_props: Props): any {}
${PREFIX()}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test.run();
