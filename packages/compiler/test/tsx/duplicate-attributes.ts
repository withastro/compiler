import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

test('basic duplicate quoted attributes - last wins', async () => {
	const input = `<div class="foo" class="bar"></div>`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.match(code, 'class="bar"');
	assert.not.match(code, 'class="foo"');
});

test('multiple duplicates - last of many wins', async () => {
	const input = `<div id="a" id="b" id="c"></div>`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.match(code, 'id="c"');
	assert.not.match(code, 'id="a"');
	assert.not.match(code, 'id="b"');
});

test('duplicate expression attributes', async () => {
	const input = '<div class={foo} class={bar}></div>';
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.match(code, 'class={bar}');
	assert.not.match(code, 'class={foo}');
});

test('duplicate empty attributes', async () => {
	const input = '<div disabled disabled></div>';
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	// Should only appear once
	const matches = code.match(/disabled/g);
	assert.ok(matches, 'disabled should be present');
	assert.is(matches.length, 1, 'disabled should appear exactly once');
});

test('duplicate template literal attributes', async () => {
	const input = '<div class={`${a}`} class={`${b}`}></div>';
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.match(code, 'class={`${b}`}');
	assert.not.match(code, 'class={`${a}`}');
});

test('mixed attribute types for same key', async () => {
	const input = `<div class="foo" class={bar}></div>`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.match(code, 'class={bar}');
	assert.not.match(code, 'class="foo"');
});

test('interleaved duplicates preserve order', async () => {
	const input = `<div a="1" b="2" a="3" c="4"></div>`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	// Should output in order: b="2" a="3" c="4"
	const aIndex = code.indexOf('a="3"');
	const bIndex = code.indexOf('b="2"');
	const cIndex = code.indexOf('c="4"');
	assert.ok(aIndex > 0, 'a="3" should be present');
	assert.ok(bIndex > 0, 'b="2" should be present');
	assert.ok(cIndex > 0, 'c="4" should be present');
	assert.ok(bIndex < aIndex, 'b should come before a');
	assert.ok(aIndex < cIndex, 'a should come before c');
	assert.not.match(code, 'a="1"', 'a="1" should not be present');
});

test('spread attributes with later overrides', async () => {
	const input = `<div class="foo" {...props} class="bar"></div>`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	// Spread should just be {...props} in TSX
	assert.match(code, '{...props}');
	assert.match(code, 'class="bar"');
	assert.not.match(code, 'class="foo"');
});

test('complex spread scenario', async () => {
	const input = `<div a="1" {...props} a="2" a="3"></div>`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	// Named attrs should be deduplicated to just a="3"
	assert.match(code, 'a="3"');
	assert.not.match(code, 'a="1"');
	assert.not.match(code, 'a="2"');
	// Spread remains as-is
	assert.match(code, '{...props}');
});

test('multiple spreads with different exclusions', async () => {
	const input = `<div a="1" {...props1} b="2" {...props2} a="3"></div>`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	// Both spreads remain
	assert.match(code, '{...props1}');
	assert.match(code, '{...props2}');
	// Named attrs should be b="2" and a="3"
	assert.match(code, 'b="2"');
	assert.match(code, 'a="3"');
	assert.not.match(code, 'a="1"');
});

test('namespaced attributes (SVG)', async () => {
	const input = `<svg xlink:href="a" xlink:href="b"></svg>`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.match(code, 'xlink:href="b"');
	assert.not.match(code, 'xlink:href="a"');
});

test('duplicate attributes on components', async () => {
	const input = `<Component prop="a" prop="b"></Component>`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.match(code, 'prop="b"');
	assert.not.match(code, 'prop="a"');
});

test('case sensitivity - different cases are different attributes', async () => {
	const input = `<div class="foo" CLASS="bar"></div>`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	// Both should remain (case-sensitive)
	assert.match(code, 'class="foo"');
	assert.match(code, 'CLASS="bar"');
});

test('spread before all named attrs', async () => {
	const input = `<div {...props} class="override"></div>`;
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	// Spread remains as-is in TSX
	assert.match(code, '{...props}');
	assert.match(code, 'class="override"');
});

test('spread with no conflicts', async () => {
	const input = '<div {...props}></div>';
	const { code } = await convertToTSX(input, { sourcemap: 'external' });
	assert.match(code, '{...props}');
});

test.run();
