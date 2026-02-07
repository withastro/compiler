import { convertToTSX } from '@astrojs/compiler';
import { TraceMap, generatedPositionFor } from '@jridgewell/trace-mapping';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testTSXSourcemap } from '../utils.js';

describe('tsx-sourcemaps/tags', { skip: true }, () => {
	it('tag close', async () => {
		const input = '<Hello></Hello>';
		const output = await testTSXSourcemap(input, '>');

		assert.deepStrictEqual(output, {
			line: 1,
			column: 6,
			source: 'index.astro',
			name: null,
		});
	});

	it('tag with spaces', async () => {
		const input = '<Button      ></Button>';
		const { map } = await convertToTSX(input, { sourcemap: 'both', filename: 'index.astro' });
		const tracer = new TraceMap(map as any);

		const generated = generatedPositionFor(tracer, { source: 'index.astro', line: 1, column: 14 });

		assert.deepStrictEqual(generated, {
			line: 4,
			column: 9,
		});
	});
});
