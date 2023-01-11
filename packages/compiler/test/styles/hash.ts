import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE_A = `
<style>
  h1 { color: red; }
</style>

<h1>Hello world!</h1>
`;
const FIXTURE_B = `
<style>
  h1 { color: blue; }
</style>

<h1>Hello world!</h1>
`;
const FIXTURE_C = `
<style>
  h1 { color: red; }
</style>

<script>console.log("Hello world")</script>
`;
const FIXTURE_D = `
<style>
  h1 { color: red; }
</style>

<script>console.log("Hello world!")</script>
`;

const scopes: string[] = [];
test.before(async () => {
  const [{ scope: a }, { scope: b }, { scope: c }, { scope: d }] = await Promise.all([FIXTURE_A, FIXTURE_B, FIXTURE_C, FIXTURE_D].map((source) => transform(source)));
  scopes.push(a, b, c, d);
});

test('hash changes when content outside of style change', () => {
  const [, b, c] = scopes;
  assert.not.equal(b, c, 'Expected scopes to not be equal');
});

test('hash changes when scripts change', () => {
  const [, , c, d] = scopes;
  assert.not.equal(c, d, 'Expected scopes to not be equal');
});

test.run();
