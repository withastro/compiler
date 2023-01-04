import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';
import { preprocessStyle } from '../utils';

const FIXTURE = `
---
let color = 'red';
---

<style lang="scss" define:vars={{ color }}>
  div {
    color: var(--color);
  }
</style>

<div>Hello world!</div>

<div>Ahhh</div>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    sourcemap: true,
    preprocessStyle,
  });
});

test('does not include define:vars in generated markup', () => {
  assert.ok(!result.code.includes('STYLES'));
  assert.equal(result.css.length, 1);
});

test.run();
