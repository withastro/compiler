import { test } from 'uvu';
import * as assert from 'uvu/assert';
import * as fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { transform } from '@astrojs/compiler';

import { SourceMapConsumer } from 'source-map';

const fixtureURL = new URL('../../fixtures/compact-sourcemap/index.astro', import.meta.url);
let result;
test.before(async () => {
  result = await transform(fs.readFileSync(fileURLToPath(fixtureURL)).toString(), {
    sourcefile: fileURLToPath(fixtureURL),
    sourcemap: 'external',
  });
});

test('Includes external sourcemap', async () => {
  assert.ok(result.map, 'Has sourcemap');
  const { code: out } = result;
  const map = await new SourceMapConsumer(JSON.parse(result.map));

  const outLines = out.split('\n');
  const outLine = outLines.findIndex((ln) => ln.includes('Hello'));
  let outColumn = outLines[outLine].indexOf('Hello');

  const { source, line, column } = map.originalPositionFor({ line: outLine + 1, column: outColumn + 1 });
  assert.equal(source, fileURLToPath(fixtureURL));
  assert.equal(line, 5);
  assert.equal(column, 2);
});

test.run();
