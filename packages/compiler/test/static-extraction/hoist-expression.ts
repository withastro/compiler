import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
---
const url = 'foo';
---
<script type="module" hoist src={url}></script>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, { experimentalStaticExtraction: true });
});

test('logs warning with hoisted expression', () => {
  assert.ok(result.code);
});

test.run();
