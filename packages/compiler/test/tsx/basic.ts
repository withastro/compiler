import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { convertToTSX } from '@astrojs/compiler';

test('basic', async () => {
  const input = `
---
let value = 'world';
---

<h1 name="value" empty {shorthand} expression={true} literal=\`tags\`>Hello {value}</h1>
<div></div>
`;
  const output = `
let value = 'world';


<Fragment>
<h1 name="value" empty shorthand={shorthand} expression={true} literal={\`tags\`}>Hello {value}</h1>
<div></div>
</Fragment>

export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('named export', async () => {
  const input = `
---
let value = 'world';
---

<h1 name="value" empty {shorthand} expression={true} literal=\`tags\`>Hello {value}</h1>
<div></div>
`;
  const output = `
let value = 'world';


<Fragment>
<h1 name="value" empty shorthand={shorthand} expression={true} literal={\`tags\`}>Hello {value}</h1>
<div></div>
</Fragment>

export default function Test__AstroComponent_(_props: Record<string, any>): any {}
`;
  const { code } = await convertToTSX(input, { sourcefile: '/Users/nmoo/test.astro', sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('moves @attributes to spread', async () => {
  const input = `<div @click={() => {}} name="value"></div>`;
  const output = `<Fragment>
<div name="value" {...{"@click":(() => {})}}></div>
</Fragment>

export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('add trailing semicolon to frontmatter', async () => {
  const input = `
---
console.log("hello")
---

{hello}
`;
  const output = `
console.log("hello")


;<Fragment>
{hello}
</Fragment>

export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('moves attributes with dots in them to spread', async () => {
  const input = `<div x-on:keyup.shift.enter="alert('Astro')" name="value"></div>`;
  const output = `<Fragment>
<div name="value" {...{"x-on:keyup.shift.enter":"alert('Astro')"}}></div>
</Fragment>

export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test('preserves unclosed tags', async () => {
  const input = `<components.`;
  const output = `<Fragment>
<components.
</Fragment>

export default function __AstroComponent_(_props: Record<string, any>): any {}
`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test.run();
