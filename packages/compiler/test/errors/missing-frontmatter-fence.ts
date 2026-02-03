import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

// Missing opening frontmatter fence - only has closing ---
const FIXTURE = `import BaseLayout from '@/layouts/BaseLayout.astro';
import { getCollection } from 'astro:content';
const posts = await getCollection('blog');
---
<BaseLayout title="Crash Test">
  <h1>{posts.length}</h1>
</BaseLayout>`;

let result: TransformResult;
test.before(async () => {
	result = await transform(FIXTURE, {
		filename: '/src/pages/checkthis.astro',
	});
});

test('missing opening frontmatter fence reports error instead of panic', () => {
	assert.ok(Array.isArray(result.diagnostics));
	assert.is(result.diagnostics.length, 1);
	assert.is(result.diagnostics[0].code, 1006);
	assert.is(
		result.diagnostics[0].text,
		'The closing frontmatter fence (---) is missing an opening fence'
	);
	assert.is(
		result.diagnostics[0].hint,
		'Add --- at the beginning of your file before any import statements or code'
	);
	// Verify the error location points to the closing --- fence
	const loc = result.diagnostics[0].location;
	// The line number should point to the line containing ---
	assert.is(FIXTURE.split('\n')[loc.line - 1], '---');
	// The column and length should extract exactly the --- characters
	assert.is(
		FIXTURE.split('\n')[loc.line - 1].slice(loc.column - 1, loc.column - 1 + loc.length),
		'---'
	);
});

test.run();
