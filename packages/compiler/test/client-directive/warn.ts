import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
<script client:load></script>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, { experimentalStaticExtraction: true });
});

test('logs a warning for using a client directive', () => {
  assert.ok(result.code);
});

test.run();
