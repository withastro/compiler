import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

test('basic', async () => {
  const result = await transform(`    <div>Hello {value}</div>      `, {
    mode: 'production',
  });
  assert.ok(result.code.includes('$$render`<div>Hello ${value}</div>`'), `Expected minfied result to match fixture`);
});

test('preserve pre', async () => {
  const result = await transform(`<pre>  !  </pre>`, {
    mode: 'production',
  });
  assert.ok(result.code.includes('$$render`<pre>  !  </pre>`'), `Expected minfied result to match fixture`);
});

test('preserve is:raw', async () => {
  const result = await transform(`<div is:raw>  !  </div>`, {
    mode: 'production',
  });
  assert.ok(result.code.includes('$$render`<div>  !  </div>`'), `Expected minfied result to match fixture`);
});

test('preserve Markdown', async () => {
  const result = await transform(`<Markdown>  !  </Markdown>`, {
    mode: 'production',
  });
  assert.ok(result.code.includes('$$render`  !  `'), `Expected minfied result to match fixture`);
});

test('collapse inline', async () => {
  const result = await transform(`<span>If <strong>inline</strong></span>`, {
    mode: 'production',
  });
  assert.ok(result.code.includes('$$render`<span>If <strong>inline</strong></span>`'), `Expected minfied result to match fixture`);
});

test('collapse only child', async () => {
  const result = await transform(`<span> inline </span>`, {
    mode: 'production',
  });
  assert.ok(result.code.includes('$$render`<span>inline</span>`'), `Expected minfied result to match fixture`);
});

test('collapse expression', async () => {
  const result = await transform(`<span> inline { expression }</span>`, {
    mode: 'production',
  });
  assert.ok(result.code.includes('$$render`<span>inline ${expression}</span>`'), `Expected minfied result to match fixture`);
});
