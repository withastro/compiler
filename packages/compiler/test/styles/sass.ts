import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';
import { preprocessStyle } from '../utils';

const FIXTURE = `
---
let value = 'world';
---

<style lang="scss" define:vars={{ a: 0 }}>
$color: red;

div {
  color: $color;
}
</style>

<div>Hello world!</div>

<div>Ahhh</div>

<style lang="scss">
$color: green;
div {
  color: $color;
}
</style>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    sourcemap: true,
    preprocessStyle,
  });
});

test('transforms scss one', () => {
  assert.match(result.code, 'color:red', 'Expected "color:red" to be present.');
});

test('transforms scss two', () => {
  assert.match(result.code, 'color:green', 'Expected "color:green" to be present.');
});

test.run();
