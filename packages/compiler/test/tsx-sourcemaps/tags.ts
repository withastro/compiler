import { convertToTSX } from '@astrojs/compiler';
import { TraceMap, generatedPositionFor } from '@jridgewell/trace-mapping';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testTSXSourcemap } from '../utils';

test('tag close', async () => {
	const input = '<Hello></Hello>';
	const output = await testTSXSourcemap(input, '>');

	assert.equal(output, {
		line: 1,
		column: 6,
		source: 'index.astro',
		name: null,
	});
});

test('tag with spaces', async () => {
	const input = '<Button      ></Button>';
	const { map } = await convertToTSX(input, { sourcemap: 'both', filename: 'index.astro' });
	const tracer = new TraceMap(map);

	const generated = generatedPositionFor(tracer, { source: 'index.astro', line: 1, column: 14 });

	assert.equal(generated, {
		line: 4,
		column: 9,
	});
});

test.run();
