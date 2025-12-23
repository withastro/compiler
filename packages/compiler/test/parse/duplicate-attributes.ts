import { parse } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import type { ElementNode } from '../../types.js';

test('duplicate attributes in AST - basic', async () => {
	const input = `<div class="foo" class="bar"></div>`;
	const { ast } = await parse(input);
	const element = ast.children[0] as ElementNode;

	assert.equal(element.attributes.length, 1, 'Should have only 1 attribute');
	assert.equal(element.attributes[0].name, 'class', 'Attribute should be "class"');
	assert.equal(element.attributes[0].value, 'bar', 'Value should be "bar" (last wins)');
});

test('duplicate attributes in AST - multiple duplicates', async () => {
	const input = `<div id="a" id="b" id="c"></div>`;
	const { ast } = await parse(input);
	const element = ast.children[0] as ElementNode;

	assert.equal(element.attributes.length, 1, 'Should have only 1 attribute');
	assert.equal(element.attributes[0].name, 'id', 'Attribute should be "id"');
	assert.equal(element.attributes[0].value, 'c', 'Value should be "c" (last of many wins)');
});

test('duplicate attributes in AST - interleaved', async () => {
	const input = `<div a="1" b="2" a="3" c="4"></div>`;
	const { ast } = await parse(input);
	const element = ast.children[0] as ElementNode;

	assert.equal(element.attributes.length, 3, 'Should have 3 unique attributes');
	assert.equal(element.attributes[0].name, 'b', 'First should be "b"');
	assert.equal(element.attributes[0].value, '2');
	assert.equal(element.attributes[1].name, 'a', 'Second should be "a"');
	assert.equal(element.attributes[1].value, '3', 'Value should be "3" (last occurrence)');
	assert.equal(element.attributes[2].name, 'c', 'Third should be "c"');
	assert.equal(element.attributes[2].value, '4');
});

test('duplicate attributes with spread - spread is kept', async () => {
	const input = `<div class="foo" {...props} class="bar"></div>`;
	const { ast } = await parse(input);
	const element = ast.children[0] as ElementNode;

	assert.equal(element.attributes.length, 2, 'Should have 2 items (spread + class)');
	assert.equal(element.attributes[0].kind, 'spread', 'First should be spread');
	assert.equal(element.attributes[1].name, 'class', 'Second should be "class"');
	assert.equal(element.attributes[1].value, 'bar', 'Value should be "bar"');
});

test('duplicate namespaced attributes', async () => {
	const input = `<svg xlink:href="a" xlink:href="b"></svg>`;
	const { ast } = await parse(input);
	const element = ast.children[0] as ElementNode;

	assert.equal(element.attributes.length, 1, 'Should have only 1 attribute');
	assert.equal(element.attributes[0].name, 'xlink:href', 'Attribute should be "xlink:href"');
	assert.equal(element.attributes[0].value, 'b', 'Value should be "b" (last wins)');
});

test('duplicate empty attributes', async () => {
	const input = `<div disabled disabled></div>`;
	const { ast } = await parse(input);
	const element = ast.children[0] as ElementNode;

	assert.equal(element.attributes.length, 1, 'Should have only 1 attribute');
	assert.equal(element.attributes[0].name, 'disabled', 'Attribute should be "disabled"');
	assert.equal(element.attributes[0].kind, 'empty', 'Should be empty attribute');
});

test('case sensitivity - different cases kept separate', async () => {
	const input = `<div class="foo" CLASS="bar"></div>`;
	const { ast } = await parse(input);
	const element = ast.children[0] as ElementNode;

	assert.equal(element.attributes.length, 2, 'Should have 2 attributes (case-sensitive)');
	assert.equal(element.attributes[0].name, 'class');
	assert.equal(element.attributes[0].value, 'foo');
	assert.equal(element.attributes[1].name, 'CLASS');
	assert.equal(element.attributes[1].value, 'bar');
});

test('duplicate expression attributes', async () => {
	const input = `<div class={foo} class={bar}></div>`;
	const { ast } = await parse(input);
	const element = ast.children[0] as ElementNode;

	assert.equal(element.attributes.length, 1, 'Should have only 1 attribute');
	assert.equal(element.attributes[0].name, 'class');
	assert.equal(element.attributes[0].kind, 'expression');
	assert.equal(element.attributes[0].value, 'bar', 'Value should be "bar" (last wins)');
});

test.run();
