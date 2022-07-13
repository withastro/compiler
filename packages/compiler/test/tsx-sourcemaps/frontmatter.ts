import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { convertToTSX } from '@astrojs/compiler';
import { generatedPositionFor, TraceMap } from '@jridgewell/trace-mapping';

test('frontmatter', async () => {
  const input = `---
dontExist
---
`;

  const { map } = await convertToTSX(input);
  const tracer = new TraceMap(map);

  const traced = generatedPositionFor(tracer, { source: '<stdin>', line: 2, column: 0 });
  assert.equal(traced, {
    line: 1,
    column: 0,
  });
});

test.run();
