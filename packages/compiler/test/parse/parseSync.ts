import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { initialize, parseSync } from '@astrojs/compiler';

const FIXTURE = `
---
let value = 'world';
---

<h1 name="value" empty {shorthand} expression={true} literal=\`tags\`>Hello {value}</h1>
<div></div>
`;

let result;
test.before(async () => {
  await initialize();
  result = parseSync(FIXTURE);
});

test('ast', () => {
  assert.type(result, 'object', `Expected "parse" to return an object!`);
  assert.equal(result.ast.type, 'root', `Expected "ast" root node to be of type "root"`);
});

test('frontmatter', () => {
  const [frontmatter] = result.ast.children;
  assert.equal(frontmatter.type, 'frontmatter', `Expected first child node to be of type "frontmatter"`);
});

test('element', () => {
  const [, element] = result.ast.children;
  assert.equal(element.type, 'element', `Expected first child node to be of type "element"`);
});

test('element with no attributes', () => {
  const [, , , element] = result.ast.children;
  assert.equal(element.attributes, [], `Expected the "attributes" property to be an empty array`);
});

test('element with no children', () => {
  const [, , , element] = result.ast.children;
  assert.equal(element.children, [], `Expected the "children" property to be an empty array`);
});

test.run();
