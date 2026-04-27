import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

async function jsxMinify(input: string) {
	const code = (await transform(input, { compact: 'jsx' })).code;
	return code.replace('${$$maybeRenderHead($$result)}', '');
}

test('jsx: basic text', async () => {
	assert.match(
		await jsxMinify('<div>Hello {value}!</div>'),
		'$$render`<div>Hello ${value}!</div>`'
	);
});

test('jsx: single line preserves spaces', async () => {
	assert.match(await jsxMinify('<div>  Hello  </div>'), '$$render`<div>  Hello  </div>`');
});

test('jsx: multiline strips indentation', async () => {
	assert.match(
		await jsxMinify('<div>\n  Hello\n  World\n</div>'),
		'$$render`<div>Hello World</div>`'
	);
});

test('jsx: tabs preserved on single line', async () => {
	assert.match(await jsxMinify('<div>\tHello\t</div>'), '$$render`<div>\tHello\t</div>`');
});

test('jsx: whitespace-only multiline is removed', async () => {
	assert.match(await jsxMinify('<div>\n  <p>text</p>\n</div>'), '$$render`<div><p>text</p></div>`');
});

test('jsx: preservation', async () => {
	assert.match(
		await jsxMinify('<pre>  Hello\n  World  </pre>'),
		'$$render`<pre>  Hello\n  World  </pre>`'
	);
	assert.match(
		await jsxMinify('<textarea>  Hello\n  World  </textarea>'),
		'$$render`<textarea>  Hello\n  World  </textarea>`'
	);
});

test('jsx: expressions', async () => {
	assert.match(await jsxMinify('<div>hello {x}</div>'), '$$render`<div>hello ${x}</div>`');
	assert.match(await jsxMinify('<div>{x} hello</div>'), '$$render`<div>${x} hello</div>`');
});

test('jsx: expression trimming', async () => {
	assert.match(
		await jsxMinify('<div>{\n  expression\n}</div>'),
		'$$render`<div>${expression}</div>`'
	);
});

test('jsx: typical component children', async () => {
	assert.match(
		await jsxMinify('<div>\n  <h1>Title</h1>\n  <p>Content</p>\n</div>'),
		'$$render`<div><h1>Title</h1><p>Content</p></div>`'
	);
});

test.run();
