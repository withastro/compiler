import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `---
// eslint-disable-next-line no-undef
export interface Props extends astroHTML.JSX.HTMLAttributes {}

const props = { ...Astro.props } as Props;
---

<body class:list={props['class:list']}>
  <slot></slot>
</body>`;

let result;
test.before(async () => {
  result = await transform(FIXTURE);
});

test('< and > as raw text', () => {
  assert.ok(result.code, 'Expected to compile');
  assert.match(result.code, /\/\/ eslint-disable-next-line no-undef\n/g);
  assert.equal(result.diagnostics.length, 0, 'Expected no diagnostics');
});

test.run();
