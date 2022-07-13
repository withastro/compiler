import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { convertToTSX } from '@astrojs/compiler';
import { generatedPositionFor, TraceMap } from '@jridgewell/trace-mapping';

test('template expression basic', async () => {
  const input = `<div>
{dontExist}
</div>
`;

  const { map } = await convertToTSX(input);
  const tracer = new TraceMap(map);

  const traced = generatedPositionFor(tracer, { source: '<stdin>', line: 2, column: 1 });
  assert.equal(traced, {
    line: 3,
    column: 1,
  });
});

test('template expression has dot', async () => {
  const input = `<div>
{console.log(hey)}
</div>
`;

  const { map } = await convertToTSX(input);
  const tracer = new TraceMap(map);

  const traced = generatedPositionFor(tracer, { source: '<stdin>', line: 2, column: 13 });
  assert.equal(traced, {
    line: 3,
    column: 13,
  });
});

test('template expression with addition', async () => {
  const input = `{"hello" + hey}`;

  const { map } = await convertToTSX(input);
  const tracer = new TraceMap(map);

  const traced = generatedPositionFor(tracer, { source: '<stdin>', line: 1, column: 10 });
  assert.equal(traced, {
    line: 2,
    column: 10,
  });
});

test('html attribute', async () => {
  const input = `<svg color="#000"></svg>`;

  const { map } = await convertToTSX(input);
  const tracer = new TraceMap(map);

  const traced = generatedPositionFor(tracer, { source: '<stdin>', line: 1, column: 6 });
  assert.equal(traced, {
    line: 2,
    column: 6,
  });
});

test('complex template expression', async () => {
  const input = `{[].map(item => {
return <div>{items}</div>
})}`;

  const { map } = await convertToTSX(input);
  const tracer = new TraceMap(map);

  const item = generatedPositionFor(tracer, { source: '<stdin>', line: 1, column: 7 });
  const items = generatedPositionFor(tracer, { source: '<stdin>', line: 2, column: 12 });

  assert.equal(item, {
    line: 2,
    column: 7,
  });
  assert.equal(items, {
    line: 3,
    column: 24,
  });
});

test.run();
