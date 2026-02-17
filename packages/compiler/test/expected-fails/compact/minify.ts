import { transform } from '@astrojs/compiler-rs';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

async function minify(input: string) {
	const code = (await transform(input, { compact: true })).code;
	return code.replace('${$$maybeRenderHead($$result)}', '');
}

describe('compact/minify', { skip: true }, () => {
	it('basic', async () => {
		assert.ok(
			(await minify('    <div>Hello {value}!</div>      ')).includes(
				'$$render`<div>Hello ${value}!</div>`',
			),
		);
		assert.ok(
			(await minify('    <div> Hello {value}! </div>      ')).includes(
				'$$render`<div> Hello ${value}! </div>`',
			),
		);
	});

	it('preservation', async () => {
		assert.ok((await minify('<pre>  !  </pre>')).includes('$$render`<pre>  !  </pre>`'));
		assert.ok((await minify('<div is:raw>  !  </div>')).includes('$$render`<div>  !  </div>`'));
		assert.ok((await minify('<Markdown is:raw>  !  </Markdown>')).includes('$$render`  !  `'));
	});

	it('collapsing', async () => {
		assert.ok((await minify('<span> inline </span>')).includes('$$render`<span> inline </span>`'));
		assert.ok(
			(await minify('<span>\n inline \t{\t expression \t}</span>')).includes(
				'$$render`<span>\ninline ${expression}</span>`',
			),
		);
		assert.ok(
			(await minify('<span> inline { expression }</span>')).includes(
				'$$render`<span> inline ${expression}</span>`',
			),
		);
	});

	it('space normalization between attributes', async () => {
		assert.ok((await minify('<p title="bar">foo</p>')).includes('<p title="bar">foo</p>'));
		assert.ok((await minify('<img src="test"/>')).includes('<img src="test">'));
		assert.ok((await minify('<p title = "bar">foo</p>')).includes('<p title="bar">foo</p>'));
		assert.ok(
			(await minify('<p title\n\n\t  =\n     "bar">foo</p>')).includes('<p title="bar">foo</p>'),
		);
		assert.ok((await minify('<img src="test" \n\t />')).includes('<img src="test">'));
		assert.ok(
			(await minify('<input title="bar"       id="boo"    value="hello world">')).includes(
				'<input title="bar" id="boo" value="hello world">',
			),
		);
	});

	it('space normalization around text', async () => {
		assert.ok((await minify('   <p>blah</p>\n\n\n   ')).includes('<p>blah</p>'));
		assert.ok((await minify('<p>foo <img> bar</p>')).includes('<p>foo <img> bar</p>'));
		assert.ok((await minify('<p>foo<img>bar</p>')).includes('<p>foo<img>bar</p>'));
		assert.ok((await minify('<p>foo <img>bar</p>')).includes('<p>foo <img>bar</p>'));
		assert.ok((await minify('<p>foo<img> bar</p>')).includes('<p>foo<img> bar</p>'));
		assert.ok((await minify('<p>foo <wbr> bar</p>')).includes('<p>foo <wbr> bar</p>'));
		assert.ok((await minify('<p>foo<wbr>bar</p>')).includes('<p>foo<wbr>bar</p>'));
		assert.ok((await minify('<p>foo <wbr>bar</p>')).includes('<p>foo <wbr>bar</p>'));
		assert.ok((await minify('<p>foo<wbr> bar</p>')).includes('<p>foo<wbr> bar</p>'));
		assert.ok(
			(await minify('<p>foo <wbr baz moo=""> bar</p>')).includes('<p>foo <wbr baz moo=""> bar</p>'),
		);
		assert.ok(
			(await minify('<p>foo<wbr baz moo="">bar</p>')).includes('<p>foo<wbr baz moo="">bar</p>'),
		);
		assert.ok(
			(await minify('<p>foo <wbr baz moo="">bar</p>')).includes('<p>foo <wbr baz moo="">bar</p>'),
		);
		assert.ok(
			(await minify('<p>foo<wbr baz moo=""> bar</p>')).includes('<p>foo<wbr baz moo=""> bar</p>'),
		);
		assert.ok(
			(await minify('<p>  <a href="#">  <code>foo</code></a> bar</p>')).includes(
				'<p> <a href="#"> <code>foo</code></a> bar</p>',
			),
		);
		assert.ok(
			(await minify('<p><a href="#"><code>foo  </code></a> bar</p>')).includes(
				'<p><a href="#"><code>foo </code></a> bar</p>',
			),
		);
		assert.ok(
			(await minify('<p>  <a href="#">  <code>   foo</code></a> bar   </p>')).includes(
				'<p> <a href="#"> <code> foo</code></a> bar </p>',
			),
		);
		assert.ok(
			(await minify('<div> Empty <!-- or --> not </div>')).includes(
				'<div> Empty <!-- or --> not </div>',
			),
		);
		assert.ok(
			(await minify('<div> a <input><!-- b --> c </div>')).includes(
				'<div> a <input><!-- b --> c </div>',
			),
		);
		await Promise.all(
			[
				'a',
				'abbr',
				'acronym',
				'b',
				'big',
				'del',
				'em',
				'font',
				'i',
				'ins',
				'kbd',
				'mark',
				's',
				'samp',
				'small',
				'span',
				'strike',
				'strong',
				'sub',
				'sup',
				'time',
				'tt',
				'u',
				'var',
			].map(async (el) => {
				const [open, close] = [`<${el}>`, `</${el}>`];
				assert.ok(
					(await minify(`foo ${open}baz${close} bar`)).includes(`foo ${open}baz${close} bar`),
				);
				assert.ok((await minify(`foo${open}baz${close}bar`)).includes(`foo${open}baz${close}bar`));
				assert.ok(
					(await minify(`foo ${open}baz${close}bar`)).includes(`foo ${open}baz${close}bar`),
				);
				assert.ok(
					(await minify(`foo${open}baz${close} bar`)).includes(`foo${open}baz${close} bar`),
				);
				assert.ok(
					(await minify(`foo ${open} baz ${close} bar`)).includes(`foo ${open} baz ${close} bar`),
				);
				assert.ok(
					(await minify(`foo${open} baz ${close}bar`)).includes(`foo${open} baz ${close}bar`),
				);
				assert.ok(
					(await minify(`foo ${open} baz ${close}bar`)).includes(`foo ${open} baz ${close}bar`),
				);
				assert.ok(
					(await minify(`foo${open} baz ${close} bar`)).includes(`foo${open} baz ${close} bar`),
				);
				assert.ok(
					(await minify(`<div>foo ${open}baz${close} bar</div>`)).includes(
						`<div>foo ${open}baz${close} bar</div>`,
					),
				);
				assert.ok(
					(await minify(`<div>foo${open}baz${close}bar</div>`)).includes(
						`<div>foo${open}baz${close}bar</div>`,
					),
				);
				assert.ok(
					(await minify(`<div>foo ${open}baz${close}bar</div>`)).includes(
						`<div>foo ${open}baz${close}bar</div>`,
					),
				);
				assert.ok(
					(await minify(`<div>foo${open}baz${close} bar</div>`)).includes(
						`<div>foo${open}baz${close} bar</div>`,
					),
				);
				assert.ok(
					(await minify(`<div>foo ${open} baz ${close} bar</div>`)).includes(
						`<div>foo ${open} baz ${close} bar</div>`,
					),
				);
				assert.ok(
					(await minify(`<div>foo${open} baz ${close}bar</div>`)).includes(
						`<div>foo${open} baz ${close}bar</div>`,
					),
				);
				assert.ok(
					(await minify(`<div>foo ${open} baz ${close}bar</div>`)).includes(
						`<div>foo ${open} baz ${close}bar</div>`,
					),
				);
				assert.ok(
					(await minify(`<div>foo${open} baz ${close} bar</div>`)).includes(
						`<div>foo${open} baz ${close} bar</div>`,
					),
				);
			}),
		);
		// Don't trim whitespace around element, but do trim within
		await Promise.all(
			['bdi', 'bdo', 'button', 'cite', 'code', 'dfn', 'math', 'q', 'rt', 'rtc', 'ruby', 'svg'].map(
				async (el) => {
					const [open, close] = [`<${el}>`, `</${el}>`];
					assert.ok(
						(await minify(`foo ${open}baz${close} bar`)).includes(`foo ${open}baz${close} bar`),
					);
					assert.ok(
						(await minify(`foo${open}baz${close}bar`)).includes(`foo${open}baz${close}bar`),
					);
					assert.ok(
						(await minify(`foo ${open}baz${close}bar`)).includes(`foo ${open}baz${close}bar`),
					);
					assert.ok(
						(await minify(`foo${open}baz${close} bar`)).includes(`foo${open}baz${close} bar`),
					);
					assert.ok(
						(await minify(`foo ${open} baz ${close} bar`)).includes(`foo ${open} baz ${close} bar`),
					);
					assert.ok(
						(await minify(`foo${open} baz ${close}bar`)).includes(`foo${open} baz ${close}bar`),
					);
					assert.ok(
						(await minify(`foo ${open} baz ${close}bar`)).includes(`foo ${open} baz ${close}bar`),
					);
					assert.ok(
						(await minify(`foo${open} baz ${close} bar`)).includes(`foo${open} baz ${close} bar`),
					);
					assert.ok(
						(await minify(`<div>foo ${open}baz${close} bar</div>`)).includes(
							`<div>foo ${open}baz${close} bar</div>`,
						),
					);
					assert.ok(
						(await minify(`<div>foo${open}baz${close}bar</div>`)).includes(
							`<div>foo${open}baz${close}bar</div>`,
						),
					);
					assert.ok(
						(await minify(`<div>foo ${open}baz${close}bar</div>`)).includes(
							`<div>foo ${open}baz${close}bar</div>`,
						),
					);
					assert.ok(
						(await minify(`<div>foo${open}baz${close} bar</div>`)).includes(
							`<div>foo${open}baz${close} bar</div>`,
						),
					);
					assert.ok(
						(await minify(`<div>foo ${open} baz ${close} bar</div>`)).includes(
							`<div>foo ${open} baz ${close} bar</div>`,
						),
					);
					assert.ok(
						(await minify(`<div>foo${open} baz ${close}bar</div>`)).includes(
							`<div>foo${open} baz ${close}bar</div>`,
						),
					);
					assert.ok(
						(await minify(`<div>foo ${open} baz ${close}bar</div>`)).includes(
							`<div>foo ${open} baz ${close}bar</div>`,
						),
					);
					assert.ok(
						(await minify(`<div>foo${open} baz ${close} bar</div>`)).includes(
							`<div>foo${open} baz ${close} bar</div>`,
						),
					);
				},
			),
		);
		await Promise.all(
			[
				['<span> foo </span>', '<span> foo </span>'],
				[' <span> foo </span> ', '<span> foo </span>'],
				['<nobr>a</nobr>', '<nobr>a</nobr>'],
				['<nobr>a </nobr>', '<nobr>a </nobr>'],
				['<nobr> a</nobr>', '<nobr> a</nobr>'],
				['<nobr> a </nobr>', '<nobr> a </nobr>'],
				['a<nobr>b</nobr>c', 'a<nobr>b</nobr>c'],
				['a<nobr>b </nobr>c', 'a<nobr>b </nobr>c'],
				['a<nobr> b</nobr>c', 'a<nobr> b</nobr>c'],
				['a<nobr> b </nobr>c', 'a<nobr> b </nobr>c'],
				['a<nobr>b</nobr> c', 'a<nobr>b</nobr> c'],
				['a<nobr>b </nobr> c', 'a<nobr>b </nobr> c'],
				['a<nobr> b</nobr> c', 'a<nobr> b</nobr> c'],
				['a<nobr> b </nobr> c', 'a<nobr> b </nobr> c'],
				['a <nobr>b</nobr>c', 'a <nobr>b</nobr>c'],
				['a <nobr>b </nobr>c', 'a <nobr>b </nobr>c'],
				['a <nobr> b</nobr>c', 'a <nobr> b</nobr>c'],
				['a <nobr> b </nobr>c', 'a <nobr> b </nobr>c'],
				['a <nobr>b</nobr> c', 'a <nobr>b</nobr> c'],
				['a <nobr>b </nobr> c', 'a <nobr>b </nobr> c'],
				['a <nobr> b</nobr> c', 'a <nobr> b</nobr> c'],
				['a <nobr> b </nobr> c', 'a <nobr> b </nobr> c'],
			].map(async ([input, output]) => {
				assert.ok((await minify(input)).includes(output));
			}),
		);
	});

	it('surrounded by newlines (astro#7401)', async () => {
		const input = '<span>foo</span>\n\t\tbar\n\t\t<span>baz</span>';
		const output = '<span>foo</span>\nbar\n<span>baz</span>';
		const result = await minify(input);

		assert.ok(result.includes(output));
	});

	it('separated by newlines (#815)', async () => {
		const input = '<p>\n\ta\n\t<span>b</span>\n\tc\n</p>';
		const output = '<p>\na\n<span>b</span>\nc\n</p>';
		const result = await minify(input);

		assert.ok(result.includes(output));
	});
});
