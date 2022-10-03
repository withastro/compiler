import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
<script client:load></script>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE);
});

test('logs a warning for using a client directive', () => {
  assert.ok(Array.isArray(result.warnings));
  assert.is(result.warnings.length, 1);
  assert.match(result.warnings[0].text, 'does not need the client:load directive');
});

test.run();
