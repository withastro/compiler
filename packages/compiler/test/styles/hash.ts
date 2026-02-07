import { transform } from '@astrojs/compiler';
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';

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

describe('styles/hash', () => {
	before(async () => {
		const [{ scope: a }, { scope: b }, { scope: c }, { scope: d }] = await Promise.all(
			[FIXTURE_A, FIXTURE_B, FIXTURE_C, FIXTURE_D].map((source) => transform(source))
		);
		scopes.push(a, b, c, d);
	});

	it('hash changes when content outside of style change', () => {
		const [, b, c] = scopes;
		assert.notDeepStrictEqual(b, c, 'Expected scopes to not be equal');
	});

	it('hash changes when scripts change', () => {
		const [, , c, d] = scopes;
		assert.notDeepStrictEqual(c, d, 'Expected scopes to not be equal');
	});
});
