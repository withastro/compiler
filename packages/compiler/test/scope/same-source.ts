import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `
---
---

<style>
div {
  background-color: blue;
  width: 50px;
  height: 50px;
}
</style>

<div />
`.trim();

function grabAstroScope(code: string) {
	const match = /astro-[0-9A-Za-z]+/.exec(code);
	if (match) {
		return match[0];
	}
	return null;
}

test('Similar components have different scoped class names', async () => {
	let result = await transform(FIXTURE, {
		normalizedFilename: '/src/pages/index.astro',
	});
	const scopeA = grabAstroScope(result.code);
	assert.ok(scopeA);

	result = await transform(FIXTURE, {
		normalizedFilename: '/src/pages/two.astro',
	});

	const scopeB = grabAstroScope(result.code);
	assert.ok(scopeB);

	assert.ok(scopeA !== scopeB, 'The scopes should not match for different files');
});

test.run();
