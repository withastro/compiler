import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { testSourcemap } from '../utils';

test('template expression basic', async () => {
  const input = `<!--Hello world-->\r\n\r\n<div>{nonexistent}</div>\r\n`;

  const output = await testSourcemap(input, 'nonexistent');
  assert.equal(output, {
    source: 'index.astro',
    line: 3,
    column: 6,
    name: null,
  });
});

test('template expression has dot', async () => {
  const input = `<!--Hello world-->\r\n\r\n<div>{console.log(hey)}</div>`;
  const output = await testSourcemap(input, 'log');
  assert.equal(output, {
    source: 'index.astro',
    line: 3,
    column: 14,
    name: null,
  });
});

test('template expression with addition', async () => {
  const input = `<!--Hello world-->\r\n{"hello" + hey}`;
  const output = await testSourcemap(input, 'hey');
  assert.equal(output, {
    source: 'index.astro',
    line: 2,
    column: 11,
    name: null,
  });
});

test('html attribute', async () => {
  const input = `<!--Hello world-->\r\n<svg color="#000"></svg>`;
  const output = await testSourcemap(input, 'color');
  assert.equal(output, {
    source: 'index.astro',
    name: null,
    line: 2,
    column: 5,
  });
});

test('complex template expression', async () => {
  const input = `<!--Hello world-->\n{[].map(ITEM => {\r\nv = "what";\r\nreturn <div>{ITEMS}</div>\r\n})}\r\n`;
  const item = await testSourcemap(input, 'ITEM');
  const items = await testSourcemap(input, 'ITEMS');
  assert.equal(item, {
    source: 'index.astro',
    name: null,
    line: 2,
    column: 8,
  });
  assert.equal(items, {
    source: 'index.astro',
    name: null,
    line: 4,
    column: 14,
  });
});

test('attributes', async () => {
  const input = `<!--Hello world-->\r\n<div className="hello" />\r\n`;
  const className = await testSourcemap(input, 'className');
  assert.equal(className, {
    source: 'index.astro',
    name: null,
    line: 2,
    column: 5,
  });
});

test('special attributes', async () => {
  const input = `<!--Hello world-->\r\n<div @on.click="fn" />`;
  const onClick = await testSourcemap(input, '@on.click');
  assert.equal(onClick, {
    source: 'index.astro',
    name: null,
    line: 2,
    column: 5,
  });
});

test.run();
