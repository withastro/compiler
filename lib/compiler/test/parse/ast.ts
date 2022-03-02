import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { parse } from '@astrojs/compiler';

const FIXTURE = `
---
let value = 'world';
---

<h1 name="value" empty {shorthand} expression={true} literal=\`tags\`>Hello {value}</h1>
`;

let result;
test.before(async () => {
  result = await parse(FIXTURE);
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

test.run();
