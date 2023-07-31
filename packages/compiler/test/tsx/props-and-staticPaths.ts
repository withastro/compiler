import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const PREFIX = (component: string = '__AstroComponent_') => `/**
 * Astro global available in all contexts in .astro files
 *
 * [Astro documentation](https://docs.astro.build/reference/api-reference/#astro-global)
*/
declare const Astro: Readonly<import('astro').AstroGlobal<Props, typeof ${component}>>`;

test('no props', async () => {
  const input = `---
type Props = Record<string, never>;
export function getStaticProps() {
  return {};
}
---

<div></div>`;
  const output =
    '\n' +
    `type Props = Record<string, never>;
export function getStaticProps() {
  return {};
}

"";<Fragment>
<div></div>
</Fragment>
export default function __AstroComponent_(_props: Props): any {}
${PREFIX()}`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test.run();
