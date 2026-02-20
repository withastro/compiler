import { convertToTSX } from '@astrojs/compiler-rs';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { TSXPrefix } from '../../utils.js';

describe('tsx/basic', { skip: true }, () => {
	it('basic', async () => {
		const input = `
---
let value = 'world';
---

<h1 name="value" empty {shorthand} expression={true} literal=\`tags\`>Hello {value}</h1>
<div></div>
`;
		const output = `${TSXPrefix}
let value = 'world';

<Fragment>
<h1 name="value" empty shorthand={shorthand} expression={true} literal={\`tags\`}>Hello {value}</h1>
<div></div>

</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('named export', async () => {
		const input = `
---
let value = 'world';
---

<h1 name="value" empty {shorthand} expression={true} literal=\`tags\`>Hello {value}</h1>
<div></div>
`;
		const output = `${TSXPrefix}
let value = 'world';

<Fragment>
<h1 name="value" empty shorthand={shorthand} expression={true} literal={\`tags\`}>Hello {value}</h1>
<div></div>

</Fragment>
export default function Test__AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, {
			filename: '/Users/nmoo/test.astro',
			sourcemap: 'external',
		});
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('moves @attributes to spread', async () => {
		const input = `<div @click={() => {}} name="value"></div>`;
		const output = `${TSXPrefix}<Fragment>
<div name="value" {...{"@click":(() => {})}}></div>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('add trailing semicolon to frontmatter', async () => {
		const input = `
---
console.log("hello")
---

{hello}
`;
		const output = `${TSXPrefix}
console.log("hello")

{};<Fragment>
{hello}

</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('add trailing semicolon to frontmatter II', async () => {
		const input = `
---
const { hello } = Astro.props
---

<div class={hello}></div>
`;
		const output = `${TSXPrefix}
const { hello } = Astro.props

{};<Fragment>
<div class={hello}></div>

</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('moves attributes with dots in them to spread', async () => {
		const input = `<div x-on:keyup.shift.enter="alert('Astro')" name="value"></div>`;
		const output = `${TSXPrefix}<Fragment>
<div name="value" {...{"x-on:keyup.shift.enter":"alert('Astro')"}}></div>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('moves attributes that starts with : to spread', async () => {
		const input = `<div :class="hey" name="value"></div>`;
		const output = `${TSXPrefix}<Fragment>
<div name="value" {...{":class":"hey"}}></div>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it("Don't move attributes to spread unnecessarily", async () => {
		const input = `<div 丽dfds_fsfdsfs aria-blarg name="value"></div>`;
		const output = `${TSXPrefix}<Fragment>
<div 丽dfds_fsfdsfs aria-blarg name="value"></div>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('preserves unclosed tags', async () => {
		const input = '<components.';
		const output = `${TSXPrefix}<Fragment>
<components.
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('template literal attribute', async () => {
		const input = '<div class=`${hello}`></div>';
		const output = `${TSXPrefix}<Fragment>
<div class={\`\${hello}\`}></div>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('unclosed tags', async () => {
		const input = `---
const myMarkdown = await import('../content/post.md');
---

<myMarkdown.`;
		const output = `${TSXPrefix}
const myMarkdown = await import('../content/post.md');

<Fragment>
<myMarkdown.
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('unclosed tags II', async () => {
		const input = `---
const myMarkdown = await import('../content/post.md');
---

<myMarkdown.
`;
		const output = `${TSXPrefix}
const myMarkdown = await import('../content/post.md');

<Fragment>
<myMarkdown.

</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('spread object', async () => {
		const input = `<DocSearch {...{ lang, labels: { modal, placeholder } }} client:only="preact" />`;
		const output = `${TSXPrefix}<Fragment>
<DocSearch {...{ lang, labels: { modal, placeholder } }} client:only="preact" />
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('spread object II', async () => {
		const input = `<MainLayout {...Astro.props}>
</MainLayout>`;
		const output = `${TSXPrefix}<Fragment>
<MainLayout {...Astro.props}>
</MainLayout>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('fragment with no name', async () => {
		const input = '<>+0123456789</>';
		const output = `${TSXPrefix}<Fragment>
<>+0123456789</>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('preserves spaces in tag', async () => {
		const input = '<Button ></Button>';
		const output = `${TSXPrefix}<Fragment>
<Button ></Button>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('preserves spaces after attributes in tag', async () => {
		const input = '<Button a="b" ></Button>';
		const output = `${TSXPrefix}<Fragment>
<Button a="b" ></Button>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('preserves spaces in tag', async () => {
		const input = '<Button      >';
		const output = `${TSXPrefix}<Fragment>
<Button ></Button>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('preserves line returns in tag by transforming to space', async () => {
		const input = `<Button
	>`;
		const output = `${TSXPrefix}<Fragment>
<Button ></Button>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});

	it('fragment with leading linebreak', async () => {
		const input = `
<>Test123</>`;
		const output = `${TSXPrefix}<Fragment>
<>Test123</>
</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
		const { code } = await convertToTSX(input, { sourcemap: 'external' });
		assert.strictEqual(code, output, 'expected code to match snapshot');
	});
});
