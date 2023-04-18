import { convertToTSX } from '@astrojs/compiler';
import { TraceMap, originalPositionFor } from '@jridgewell/trace-mapping';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const fixture = `<h1>Hello world!</h1>`;

test('default export mapped to 0:0', async () => {
  const input = fixture;
  const { map } = await convertToTSX(input, { sourcemap: 'both', filename: 'index.astro' });
  const tracer = new TraceMap(map);
  const original = originalPositionFor(tracer, { line: 4, column: 0 });

  assert.equal(original.column, 0);
  assert.equal(original.line, 1);
});

test.run();
