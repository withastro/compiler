import { type ParseResult, parse } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

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

let result: ParseResult;
test.before(async () => {
	result = await parse(FIXTURE);
});

test('orphan head', () => {
	assert.ok(result, 'able to parse');

	const [doctype, html, ...others] = result.ast.children;
	assert.equal(others.length, 1, 'Expected only three child nodes!');
	assert.equal(doctype.type, 'doctype', `Expected first child node to be of type "doctype"`);
	assert.equal(html.type, 'element', `Expected first child node to be of type "element"`);
});

test.run();
