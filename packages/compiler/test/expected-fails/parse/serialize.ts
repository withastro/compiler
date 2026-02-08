import { parse } from '@astrojs/compiler';
import { serialize } from '@astrojs/compiler/utils';
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';

const FIXTURE = `---
let value = 'world';
let content = "Testing 123";
---

<style>
  :root {
    color: red;
  }
</style>

<div>Hello {value}</div>

<h1 name="value" set:html={content} empty {shorthand} expression={true} literal=\`tags\` {...spread}>Hello {value}</h1>

<Fragment set:html={content} />

<Markdown is:raw>
  # Hello world!
</Markdown>
`;

describe('parse/serialize', { skip: true }, () => {
	let result: string;
	before(async () => {
		const { ast } = await parse(FIXTURE);
		try {
			result = serialize(ast);
		} catch (e) {
			// eslint-disable-next-line no-console
			console.log(e);
		}
	});

	it('serialize', () => {
		assert.strictEqual(typeof result, 'string', `Expected "serialize" to return an object!`);
		assert.deepStrictEqual(result, FIXTURE, 'Expected serialized output to equal input');
	});

	it('self-close elements', async () => {
		const input = '<div />';
		const { ast } = await parse(input);
		const output = serialize(ast, { selfClose: false });
		const selfClosedOutput = serialize(ast);
		assert.deepStrictEqual(output, '<div></div>', 'Expected serialized output to equal <div></div>');
		assert.deepStrictEqual(selfClosedOutput, input, `Expected serialized output to equal ${input}`);
	});

	it('raw attributes', async () => {
		const input = `<div name="value" single='quote' un=quote />`;
		const { ast } = await parse(input);
		const output = serialize(ast);
		assert.deepStrictEqual(output, input, `Expected serialized output to equal ${input}`);
	});
});
