import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { parse } from '@astrojs/compiler';

const FIXTURE = `
<main id={\`{\`}>
</main>
`;

let result;
test.before(async () => {
  result = await parse(FIXTURE);
});

test('template-literal', () => {
  assert.type(result, 'object', `Expected "parse" to return an object!`);
  assert.equal(result.ast.type, 'root', `Expected "ast" root node to be of type "root"`);
});

test('parse { template literal', () => {
  const [element] = result.ast.children;
  const [attribute] = element.attributes;
  assert.equal(attribute.value, '`{`');
});

test.run();
