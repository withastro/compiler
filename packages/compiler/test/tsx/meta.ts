import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

test('return ranges', async () => {
	const input = `---\nconsole.log("Hello!")\n---\n\n<div></div>`;
	const { metaRanges } = await convertToTSX(input, { sourcemap: 'external' });

	assert.equal(metaRanges, {
		frontmatter: {
			start: 30,
			end: 54,
		},
		body: {
			start: 68,
			end: 80,
		},
		scripts: null,
		styles: null,
	});
});

test('return ranges - no frontmatter', async () => {
	const input = '<div></div>';
	const { metaRanges } = await convertToTSX(input, { sourcemap: 'external' });

	assert.equal(metaRanges, {
		frontmatter: {
			start: 30,
			end: 30,
		},
		body: {
			start: 41,
			end: 53,
		},
		scripts: null,
		styles: null,
	});
});

test('return proper ranges with multibyte characters', async () => {
	const input = '---\n🦄\n---\n\n<div></div>';
	const { metaRanges } = await convertToTSX(input, { sourcemap: 'external' });

	assert.equal(metaRanges, {
		frontmatter: {
			start: 30,
			end: 35,
		},
		body: {
			start: 49,
			end: 61,
		},
		scripts: null,
		styles: null,
	});
});

test('extract scripts', async () => {
	const input = `<script type="module">console.log({ test: \`literal\` })</script><script type="text/partytown">console.log({ test: \`literal\` })</script><script type="application/ld+json">{"a":"b"}</script><script is:inline>console.log("hello")</script><div onload="console.log('hey')"></div><script>console.log({ test: \`literal\` })</script><script is:raw>something;</script>`;

	const { metaRanges } = await convertToTSX(input, { sourcemap: 'external' });
	assert.equal(
		metaRanges.scripts,
		[
			{
				position: {
					start: 22,
					end: 54,
				},
				type: 'module',
				content: 'console.log({ test: `literal` })',
				lang: '',
			},
			{
				position: {
					start: 93,
					end: 125,
				},
				type: 'inline',
				content: 'console.log({ test: `literal` })',
				lang: '',
			},
			{
				position: {
					start: 169,
					end: 178,
				},
				type: 'json',
				content: '{"a":"b"}',
				lang: '',
			},
			{
				position: {
					start: 205,
					end: 225,
				},
				type: 'inline',
				content: 'console.log("hello")',
				lang: '',
			},
			{
				position: {
					start: 247,
					end: 266,
				},
				type: 'event-attribute',
				content: "console.log('hey')",
				lang: '',
			},
			{
				position: {
					start: 281,
					end: 313,
				},
				type: 'processed-module',
				content: 'console.log({ test: `literal` })',
				lang: '',
			},
			{
				position: {
					start: 337,
					end: 347,
				},
				type: 'raw',
				content: 'something;',
				lang: '',
			},
		],
		'expected metaRanges.scripts to match snapshot'
	);
});

test('extract styles', async () => {
	const input = `<style>body { color: red; }</style><div style="color: blue;"></div><style lang="scss">body { color: red; }</style><style lang="pcss">body { color: red; }</style>`;

	const { metaRanges } = await convertToTSX(input, { sourcemap: 'external' });
	assert.equal(
		metaRanges.styles,
		[
			{
				position: {
					start: 7,
					end: 27,
				},
				type: 'tag',
				content: 'body { color: red; }',
				lang: 'css',
			},
			{
				position: {
					start: 47,
					end: 60,
				},
				type: 'style-attribute',
				content: 'color: blue;',
				lang: 'css',
			},
			{
				position: {
					start: 86,
					end: 106,
				},
				type: 'tag',
				content: 'body { color: red; }',
				lang: 'scss',
			},
			{
				position: {
					start: 133,
					end: 153,
				},
				type: 'tag',
				content: 'body { color: red; }',
				lang: 'pcss',
			},
		],
		'expected metaRanges.styles to match snapshot'
	);
});

test('extract scripts and styles with multibyte characters', async () => {
	const scripts = "<script>console.log('🦄')</script><script>console.log('Hey');</script>";
	const styles =
		"<style>body { background: url('🦄.png'); }</style><style>body { background: url('Hey'); }</style>";

	const input = `${scripts}${styles}`;
	const { metaRanges } = await convertToTSX(input, { sourcemap: 'external' });

	assert.equal(
		metaRanges.scripts,
		[
			{
				position: {
					start: 8,
					end: 25,
				},
				type: 'processed-module',
				content: "console.log('🦄')",
				lang: '',
			},
			{
				position: {
					start: 42,
					end: 61,
				},
				type: 'processed-module',
				content: "console.log('Hey');",
				lang: '',
			},
		],
		'expected metaRanges.scripts to match snapshot'
	);
	assert.equal(
		metaRanges.styles,
		[
			{
				position: {
					start: 77,
					end: 112,
				},
				type: 'tag',
				content: "body { background: url('🦄.png'); }",
				lang: 'css',
			},
			{
				position: {
					start: 127,
					end: 159,
				},
				type: 'tag',
				content: "body { background: url('Hey'); }",
				lang: 'css',
			},
		],
		'expected metaRanges.styles to match snapshot'
	);
});

test('extract scripts with multibyte characters II', async () => {
	// Emojis with various byte lengths (in order, 4, 3, 8, 28) and newlines, a complicated case, if you will
	const input = `🀄✂🇸🇪👩🏻‍❤️‍👩🏽<script>
	console.log("🀄✂🇸🇪👩🏻‍❤️‍👩🏽");
</script>🀄✂🇸🇪👩🏻‍❤️‍👩🏽<div onload="console.log('🀄✂🇸🇪👩🏻‍❤️‍👩🏽')"></div>`;

	const { metaRanges } = await convertToTSX(input, { sourcemap: 'external' });

	assert.equal(
		metaRanges.scripts,
		[
			{
				position: {
					start: 27,
					end: 65,
				},
				type: 'processed-module',
				content: '\n\tconsole.log("🀄✂🇸🇪👩🏻‍❤️‍👩🏽");\n',
				lang: '',
			},
			{
				position: {
					start: 106,
					end: 141,
				},
				type: 'event-attribute',
				content: "console.log('🀄✂🇸🇪👩🏻‍❤️‍👩🏽')",
				lang: '',
			},
		],
		'expected metaRanges.scripts to match snapshot'
	);
});

test.run();
