import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
<script type="module"></script>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE);
});

test('reports a hint for adding attributes to a script tag without is:inline', () => {
  assert.equal(result.diagnostics[0].severity, 4);
  assert.match(result.diagnostics[0].text, /Astro processes your script tags to allow using TypeScript and NPM packages/);
});

test.run();
