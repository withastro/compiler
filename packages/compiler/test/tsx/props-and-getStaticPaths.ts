import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

function getPrefix({
  props = `Get<InferredGetStaticPath, 'props'>`,
  component = '__AstroComponent_',
  params = `Get<InferredGetStaticPath, 'params'>`,
}: {
  props?: string;
  component?: string;
  params?: string;
} = {}) {
  return `/**
 * Astro global available in all contexts in .astro files
 *
 * [Astro documentation](https://docs.astro.build/reference/api-reference/#astro-global)
*/
declare const Astro: Readonly<import('astro').AstroGlobal<${props}, typeof ${component}${params ? `, ${params}` : ''}>>`
}

function getSuffix() {
  return `type ArrayElement<ArrayType extends readonly unknown[]> = ArrayType extends readonly (infer ElementType)[] ? ElementType : never;
type Flattened<T> = T extends Array<infer U> ? Flattened<U> : T;
type InferredGetStaticPath = Flattened<ArrayElement<Awaited<ReturnType<typeof getStaticPaths>>>>;
type Get<T, K> = T extends undefined ? undefined : K extends keyof T ? T[K] : never;`;
}

test('explicit props definition', async () => {
  const input = `---
interface Props {};
export function getStaticPaths() {
  return {};
}
---

<div></div>`;
  const output =
    '\n' +
    `interface Props {};
export function getStaticPaths() {
  return {};
}

"";<Fragment>
<div></div>
</Fragment>
export default function __AstroComponent_(_props: Props): any {}
${getSuffix()}
${getPrefix({ props: 'Props' })}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('inferred props', async () => {
  const input = `---
export function getStaticPaths() {
  return {};
}
---

<div></div>`;
  const output =
    '\n' +
    `export function getStaticPaths() {
  return {};
}

"";<Fragment>
<div></div>
</Fragment>
export default function __AstroComponent_(_props: Get<InferredGetStaticPath, 'props'>): any {}
${getSuffix()}
${getPrefix()}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test.run();
