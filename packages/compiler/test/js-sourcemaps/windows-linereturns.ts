import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
test('Windows line returns', async () => {
	const result = await transform(
		`<div class="px-10">\r\n{\r\n() => {\r\nif (style == Style.Ordered) {\r\nreturn items.map((item: string, index: number) => (\r\n// index + 1 is needed to start with 1 not 0\r\n<p>\r\n{index + 1}\r\n<Fragment set:html={item} />\r\n</p>\r\n));\r\n} else {\r\nreturn items.map((item: string) => (\r\n<Fragment set:html={item} />\r\n));\r\n}\r\n}\r\n}\r\n</div>`,
		{ sourcemap: 'both', filename: 'index.astro', resolvePath: (i: string) => i }
	);
	assert.ok(result.code, 'Expected to compile');
});

test.run();
