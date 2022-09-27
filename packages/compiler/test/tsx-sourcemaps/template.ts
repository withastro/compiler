import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testSourcemap } from '../utils';

test('template expression basic', async () => {
  const input = `<div>{nonexistent}</div>`;

  const output = await testSourcemap(input, 'nonexistent');
  assert.equal(output, {
    source: 'index.astro',
    line: 1,
    column: 6,
    name: null,
  });
});

test('template expression has dot', async () => {
  const input = `<div>{console.log(hey)}</div>`;
  const output = await testSourcemap(input, 'log');
  assert.equal(output, {
    source: 'index.astro',
    line: 1,
    column: 14,
    name: null,
  });
});

test('template expression with addition', async () => {
  const input = `{"hello" + hey}`;
  const output = await testSourcemap(input, 'hey');
  assert.equal(output, {
    source: 'index.astro',
    line: 1,
    column: 11,
    name: null,
  });
});

test('html attribute', async () => {
  const input = `<svg color="#000"></svg>`;
  const output = await testSourcemap(input, 'color');
  assert.equal(output, {
    source: 'index.astro',
    name: null,
    line: 1,
    column: 5,
  });
});

test('complex template expression', async () => {
  const input = `{[].map(ITEM => {
v = "what";
return <div>{ITEMS}</div>
})}`;
  const item = await testSourcemap(input, 'ITEM');
  const items = await testSourcemap(input, 'ITEMS');
  assert.equal(item, {
    source: 'index.astro',
    name: null,
    line: 1,
    column: 8,
  });
  assert.equal(items, {
    source: 'index.astro',
    name: null,
    line: 3,
    column: 14,
  });
});

test('attributes', async () => {
  const input = `<div className="hello" />`;
  const className = await testSourcemap(input, 'className');
  assert.equal(className, {
    source: 'index.astro',
    name: null,
    line: 1,
    column: 5,
  });
});

test('special attributes', async () => {
  const input = `<div @on.click="fn" />`;
  const onClick = await testSourcemap(input, '@on.click');
  assert.equal(onClick, {
    source: 'index.astro',
    name: null,
    line: 1,
    column: 5,
  });
});

test.run();
