import { type TransformResult, transform } from '@astrojs/compiler';
import assert from 'node:assert/strict';
import { before, describe, it } from 'node:test';

const FIXTURE = `import BaseLayout from '@/layouts/BaseLayout.astro';
import { getCollection } from 'astro:content';
const posts = await getCollection('blog');
---
<BaseLayout title="Crash Test">
  <h1>{posts.length}</h1>
</BaseLayout>`;

describe('missing-frontmatter-fence', { skip: true }, () => {
	let result: TransformResult;
	before(async () => {
		result = await transform(FIXTURE, {
			filename: '/src/pages/checkthis.astro',
		});
	});

	it('missing opening frontmatter fence reports error instead of panic', () => {
		assert.ok(Array.isArray(result.diagnostics));
		assert.strictEqual(result.diagnostics.length, 1);
		assert.strictEqual(result.diagnostics[0].code, 1006);
		assert.strictEqual(
			result.diagnostics[0].text,
			'The closing frontmatter fence (---) is missing an opening fence',
		);
		assert.strictEqual(
			result.diagnostics[0].hint,
			'Add --- at the beginning of your file before any import statements or code',
		);
		const loc = result.diagnostics[0].location;
		assert.strictEqual(FIXTURE.split('\n')[loc.line - 1], '---');
		assert.strictEqual(
			FIXTURE.split('\n')[loc.line - 1].slice(loc.column - 1, loc.column - 1 + loc.length),
			'---',
		);
	});
});
