import { type ParseResult, parse } from '@astrojs/compiler';
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';

const FIXTURE = `
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta http-equiv="X-UA-Compatible" content="IE=edge">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Document</title>
</head>
  <body>
    <h1>
      Hello world!</h1>
  </body>
</html>
`;

describe('parse/orphan-head', { skip: true }, () => {
	let result: ParseResult;
	before(async () => {
		result = await parse(FIXTURE);
	});

	it('orphan head', () => {
		assert.ok(result, 'able to parse');

		const [doctype, html, ...others] = result.ast.children;
		assert.deepStrictEqual(others.length, 1, 'Expected only three child nodes!');
		assert.deepStrictEqual(doctype.type, 'doctype', `Expected first child node to be of type "doctype"`);
		assert.deepStrictEqual(html.type, 'element', `Expected first child node to be of type "element"`);
	});
});
