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

test('extract scripts', async () => {
	const input = `<script type="module">console.log({ test: \`literal\` })</script><script type="text/partytown">console.log({ test: \`literal\` })</script><script type="application/ld+json">{"a":"b"}</script><script is:inline>console.log("hello")</script><div onload="console.log('hey')"></div>`;

	const { metaRanges } = await convertToTSX(input, { sourcemap: 'external' });
	assert.equal(
		metaRanges.scripts,
		[
			{
				position: {
					start: 22,
					end: 87,
				},
				type: 'module',
				content: 'console.log({ test: `literal` })',
			},
			{
				position: {
					start: 93,
					end: 158,
				},
				type: 'inline',
				content: 'console.log({ test: `literal` })',
			},
			{
				position: {
					start: 169,
					end: 188,
				},
				type: 'json',
				content: '{"a":"b"}',
			},
			{
				position: {
					start: 205,
					end: 246,
				},
				type: 'inline',
				content: 'console.log("hello")',
			},
			{
				position: {
					start: 247,
					end: 266,
				},
				type: 'event-attribute',
				content: "console.log('hey')",
			},
		],
		'expected metaRanges.scripts to match snapshot'
	);
});

test('extract styles', async () => {
	const input = `<style>body { color: red; }</style><div style="color: blue;"></div>`;

	const { metaRanges } = await convertToTSX(input, { sourcemap: 'external' });
	assert.equal(
		metaRanges.styles,
		[
			{
				position: {
					start: 7,
					end: 48,
				},
				type: 'tag',
				content: 'body { color: red; }',
			},
			{
				position: {
					start: 47,
					end: 60,
				},
				type: 'style-attribute',
				content: 'color: blue;',
			},
		],
		'expected metaRanges.styles to match snapshot'
	);
});

test.run();
