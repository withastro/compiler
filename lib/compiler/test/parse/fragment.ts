import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { parse } from '@astrojs/compiler';

const FIXTURE = `<>Hello</><Fragment>World</Fragment>`;

let result;
test.before(async () => {
  result = await parse(FIXTURE);
});

test('fragment shorthand', () => {
  const [first] = result.ast.children;
  assert.equal(first.type, 'fragment', 'Expected first child to be of type "fragment"');
  assert.equal(first.name, '', 'Expected first child to have name of ""');
});

test('fragment literal', () => {
  const [_, second] = result.ast.children;
  assert.equal(second.type, 'fragment', 'Expected second child to be of type "fragment"');
  assert.equal(second.name, 'Fragment', 'Expected second child to have name of ""');
});

test.run();
