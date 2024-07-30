import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `
---
const { items, emptyItems } = Astro.props;

const internal = [];
---

<!-- False -->
{false && (
  <span id="frag-false" />
)}

<!-- Null -->
{null && (
  <span id="frag-null" />
)}

<!-- True -->
{true && (
  <span id="frag-true" />
)}

<!-- Undefined -->
{false && (<span id="frag-undefined" />)}
`;

let result: TransformResult;
test.before(async () => {
	result = await transform(FIXTURE);
});

test('top-level expressions', () => {
	assert.ok(result.code, 'Can compile top-level expressions');
});

test.run();
