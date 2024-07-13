import { parse } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

test('include start and end positions', async () => {
	const input = `---
// Hello world!
---

<iframe>Hello</iframe><div></div>`;
	const { ast } = await parse(input);

	const iframe = ast.children[1];
	assert.is(iframe.name, 'iframe');
	assert.ok(iframe.position.start, 'Expected serialized output to contain a start position');
	assert.ok(iframe.position.end, 'Expected serialized output to contain an end position');
});

test('include start and end positions for comments', async () => {
	const input = `---
// Hello world!
---

<!-- prettier:ignore -->
<iframe>Hello</iframe><div></div>`;
	const { ast } = await parse(input);

	const comment = ast.children[1];
	assert.is(comment.type, 'comment');
	assert.ok(comment.position.start, 'Expected serialized output to contain a start position');
	assert.ok(comment.position.end, 'Expected serialized output to contain an end position');
});

test('include start and end positions for text', async () => {
	const input = `---
// Hello world!
---

Hello world!`;
	const { ast } = await parse(input);

	const text = ast.children[1];
	assert.is(text.type, 'text');
	assert.ok(text.position.start, 'Expected serialized output to contain a start position');
	assert.ok(text.position.end, 'Expected serialized output to contain an end position');
});

test('include start and end positions for self-closing tags', async () => {
	const input = '<input/>';
	const { ast } = await parse(input);

	const element = ast.children[0];
	assert.is(element.type, 'element');
	assert.is(element.name, 'input');
	assert.ok(element.position.start, 'Expected serialized output to contain a start position');
	assert.ok(element.position.end, 'Expected serialized output to contain an end position');
});

test('include correct start and end position for self-closing tag', async () => {
	const input = `
<!-- prettier-ignore -->
<li />`;
	const { ast } = await parse(input);

	const li = ast.children[1];
	assert.is(li.name, 'li');
	assert.ok(li.position.start, 'Expected serialized output to contain a start position');
	assert.ok(li.position.end, 'Expected serialized output to contain an end position');

	assert.equal(
		li.position.start,
		{ line: 3, column: 1, offset: 26 },
		'Expected serialized output to contain a start position'
	);
	assert.equal(
		li.position.end,
		{ line: 3, column: 6, offset: 31 },
		'Expected serialized output to contain an end position'
	);
});

test('include correct start and end position for normal closing tag', async () => {
	const input = `
<!-- prettier-ignore -->
<li></li>`;
	const { ast } = await parse(input);

	const li = ast.children[1];
	assert.is(li.name, 'li');
	assert.ok(li.position.start, 'Expected serialized output to contain a start position');
	assert.ok(li.position.end, 'Expected serialized output to contain an end position');

	assert.equal(
		li.position.start,
		{ line: 3, column: 1, offset: 26 },
		'Expected serialized output to contain a start position'
	);
	assert.equal(
		li.position.end,
		{ line: 3, column: 10, offset: 35 },
		'Expected serialized output to contain an end position'
	);
});

test('include start and end position if frontmatter is only thing in file (#802)', async () => {
	const input = `---
---`;
	const { ast } = await parse(input);

	const frontmatter = ast.children[0];
	assert.is(frontmatter.type, 'frontmatter');
	assert.ok(frontmatter.position.start, 'Expected serialized output to contain a start position');
	assert.ok(frontmatter.position.end, 'Expected serialized output to contain an end position');

	assert.equal(
		frontmatter.position.start,
		{ line: 1, column: 1, offset: 0 },
		'Expected serialized output to contain a start position'
	);
	assert.equal(
		frontmatter.position.end,
		{ line: 2, column: 4, offset: 7 },
		'Expected serialized output to contain an end position'
	);
});

test.run();
