import { parse } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import type { ElementNode } from '../../types.js';

test('preserve style tag position I', async () => {
	const input = `<html><body><h1>Hello world!</h1></body></html>
<style></style>`;
	const { ast } = await parse(input);

	const lastChildren = ast.children.at(-1) as ElementNode;

	assert.equal(lastChildren.type, 'element', 'Expected last child node to be of type "element"');
	assert.equal(lastChildren.name, 'style', 'Expected last child node to be of type "style"');
});

test('preserve style tag position II', async () => {
	const input = `<html></html>
<style></style>`;
	const { ast } = await parse(input);

	const lastChildren = ast.children.at(-1) as ElementNode;

	assert.equal(lastChildren.type, 'element', 'Expected last child node to be of type "element"');
	assert.equal(lastChildren.name, 'style', 'Expected last child node to be of type "style"');
});

test('preserve style tag position III', async () => {
	const input = `<html lang="en"><head><BaseHead /></head></html>
<style>@use "../styles/global.scss";</style>`;
	const { ast } = await parse(input);

	const lastChildren = ast.children.at(-1) as ElementNode;

	assert.equal(lastChildren.type, 'element', 'Expected last child node to be of type "element"');
	assert.equal(lastChildren.name, 'style', 'Expected last child node to be of type "style"');
	assert.equal(
		lastChildren.children[0].type,
		'text',
		'Expected last child node to be of type "text"'
	);
});

test('preserve style tag position IV', async () => {
	const input = `<html lang="en"><head><BaseHead /></head><body><Header /></body></html>
<style>@use "../styles/global.scss";</style>`;
	const { ast } = await parse(input);

	const lastChildren = ast.children.at(-1) as ElementNode;

	assert.equal(lastChildren.type, 'element', 'Expected last child node to be of type "element"');
	assert.equal(lastChildren.name, 'style', 'Expected last child node to be of type "style"');
	assert.equal(
		lastChildren.children[0].type,
		'text',
		'Expected last child node to be of type "text"'
	);
});

test('preserve style tag position V', async () => {
	const input = `<html lang="en"><head><BaseHead /></head><body><Header /></body><style>@use "../styles/global.scss";</style></html>`;
	const { ast } = await parse(input);

	const firstChild = ast.children.at(0) as ElementNode;
	const lastChild = firstChild.children.at(-1) as ElementNode;

	assert.equal(lastChild.type, 'element', 'Expected last child node to be of type "element"');
	assert.equal(lastChild.name, 'style', 'Expected last child node to be of type "style"');
	assert.equal(lastChild.children[0].type, 'text', 'Expected last child node to be of type "text"');
});
