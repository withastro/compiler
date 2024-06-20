import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { TSXPrefix } from '../utils';

function getPrefix({
  props = `ASTRO__MergeUnion<ASTRO__Get<ASTRO__InferredGetStaticPath, 'props'>>`,
  component = '__AstroComponent_',
  params = `ASTRO__Get<ASTRO__InferredGetStaticPath, 'params'>`,
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
declare const Astro: Readonly<import('astro').AstroGlobal<${props}, typeof ${component}${params ? `, ${params}` : ''}>>`;
}

function getSuffix() {
  return `type ASTRO__ArrayElement<ArrayType extends readonly unknown[]> = ArrayType extends readonly (infer ElementType)[] ? ElementType : never;
type ASTRO__Flattened<T> = T extends Array<infer U> ? ASTRO__Flattened<U> : T;
type ASTRO__InferredGetStaticPath = ASTRO__Flattened<ASTRO__ArrayElement<Awaited<ReturnType<typeof getStaticPaths>>>>;
type ASTRO__MergeUnion<T, K extends PropertyKey = T extends unknown ? keyof T : never> = T extends unknown ? T & { [P in Exclude<K, keyof T>]?: never } extends infer O ? { [P in keyof O]: O[P] } : never : never;
type ASTRO__Get<T, K> = T extends undefined ? undefined : K extends keyof T ? T[K] : never;`;
}

test('explicit props definition', async () => {
  const input = `---
interface Props {};
export function getStaticPaths() {
  return {};
}
---

<div></div>`;
  const output = `${TSXPrefix}\ninterface Props {};
export function getStaticPaths() {
  return {};
}

{};<Fragment>
<div></div>
</Fragment>
export default function __AstroComponent_(_props: Props): any {}
${getSuffix()}
${getPrefix({ props: 'Props' })}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, 'expected code to match snapshot');
});

test('inferred props', async () => {
  const input = `---
export function getStaticPaths() {
  return {};
}
---

<div></div>`;
  const output = `${TSXPrefix}\nexport function getStaticPaths() {
  return {};
}

{};<Fragment>
<div></div>
</Fragment>
export default function __AstroComponent_(_props: ASTRO__MergeUnion<ASTRO__Get<ASTRO__InferredGetStaticPath, 'props'>>): any {}
${getSuffix()}
${getPrefix()}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, 'expected code to match snapshot');
});

test.run();
