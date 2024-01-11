import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

async function minify(input: string) {
  const code = (await transform(input, { compact: true })).code;
  return code.replace('${$$maybeRenderHead($$result)}', '');
}

test('basic', async () => {
  assert.match(await minify(`    <div>Hello {value}!</div>      `), '$$render`<div>Hello ${value}!</div>`');
  assert.match(await minify(`    <div> Hello {value}! </div>      `), '$$render`<div> Hello ${value}! </div>`');
});

test('preservation', async () => {
  assert.match(await minify(`<pre>  !  </pre>`), '$$render`<pre>  !  </pre>`');
  assert.match(await minify(`<div is:raw>  !  </div>`), '$$render`<div>  !  </div>`');
  assert.match(await minify(`<Markdown is:raw>  !  </Markdown>`), '$$render`  !  `');
});

test('collapsing', async () => {
  assert.match(await minify(`<span> inline </span>`), '$$render`<span> inline </span>`');
  assert.match(await minify(`<span>\n inline \t{\t expression \t}</span>`), '$$render`<span>\ninline ${expression}</span>`');
  assert.match(await minify(`<span> inline { expression }</span>`), '$$render`<span> inline ${expression}</span>`');
});

test('space normalization between attributes', async () => {
  assert.match(await minify('<p title="bar">foo</p>'), '<p title="bar">foo</p>');
  assert.match(await minify('<img src="test"/>'), '<img src="test">');
  assert.match(await minify('<p title = "bar">foo</p>'), '<p title="bar">foo</p>');
  assert.match(await minify('<p title\n\n\t  =\n     "bar">foo</p>'), '<p title="bar">foo</p>');
  assert.match(await minify('<img src="test" \n\t />'), '<img src="test">');
  assert.match(await minify('<input title="bar"       id="boo"    value="hello world">'), '<input title="bar" id="boo" value="hello world">');
});

test('space normalization around text', async () => {
  assert.match(await minify('   <p>blah</p>\n\n\n   '), '<p>blah</p>');
  assert.match(await minify('<p>foo <img> bar</p>'), '<p>foo <img> bar</p>');
  assert.match(await minify('<p>foo<img>bar</p>'), '<p>foo<img>bar</p>');
  assert.match(await minify('<p>foo <img>bar</p>'), '<p>foo <img>bar</p>');
  assert.match(await minify('<p>foo<img> bar</p>'), '<p>foo<img> bar</p>');
  assert.match(await minify('<p>foo <wbr> bar</p>'), '<p>foo <wbr> bar</p>');
  assert.match(await minify('<p>foo<wbr>bar</p>'), '<p>foo<wbr>bar</p>');
  assert.match(await minify('<p>foo <wbr>bar</p>'), '<p>foo <wbr>bar</p>');
  assert.match(await minify('<p>foo<wbr> bar</p>'), '<p>foo<wbr> bar</p>');
  assert.match(await minify('<p>foo <wbr baz moo=""> bar</p>'), '<p>foo <wbr baz moo=""> bar</p>');
  assert.match(await minify('<p>foo<wbr baz moo="">bar</p>'), '<p>foo<wbr baz moo="">bar</p>');
  assert.match(await minify('<p>foo <wbr baz moo="">bar</p>'), '<p>foo <wbr baz moo="">bar</p>');
  assert.match(await minify('<p>foo<wbr baz moo=""> bar</p>'), '<p>foo<wbr baz moo=""> bar</p>');
  assert.match(await minify('<p>  <a href="#">  <code>foo</code></a> bar</p>'), '<p> <a href="#"> <code>foo</code></a> bar</p>');
  assert.match(await minify('<p><a href="#"><code>foo  </code></a> bar</p>'), '<p><a href="#"><code>foo </code></a> bar</p>');
  assert.match(await minify('<p>  <a href="#">  <code>   foo</code></a> bar   </p>'), '<p> <a href="#"> <code> foo</code></a> bar </p>');
  assert.match(await minify('<div> Empty <!-- or --> not </div>'), '<div> Empty <!-- or --> not </div>');
  assert.match(await minify('<div> a <input><!-- b --> c </div>'), '<div> a <input><!-- b --> c </div>');
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
      assert.match(await minify(`foo ${open}baz${close} bar`), `foo ${open}baz${close} bar`);
      assert.match(await minify(`foo${open}baz${close}bar`), `foo${open}baz${close}bar`);
      assert.match(await minify(`foo ${open}baz${close}bar`), `foo ${open}baz${close}bar`);
      assert.match(await minify(`foo${open}baz${close} bar`), `foo${open}baz${close} bar`);
      assert.match(await minify(`foo ${open} baz ${close} bar`), `foo ${open} baz ${close} bar`);
      assert.match(await minify(`foo${open} baz ${close}bar`), `foo${open} baz ${close}bar`);
      assert.match(await minify(`foo ${open} baz ${close}bar`), `foo ${open} baz ${close}bar`);
      assert.match(await minify(`foo${open} baz ${close} bar`), `foo${open} baz ${close} bar`);
      assert.match(await minify(`<div>foo ${open}baz${close} bar</div>`), `<div>foo ${open}baz${close} bar</div>`);
      assert.match(await minify(`<div>foo${open}baz${close}bar</div>`), `<div>foo${open}baz${close}bar</div>`);
      assert.match(await minify(`<div>foo ${open}baz${close}bar</div>`), `<div>foo ${open}baz${close}bar</div>`);
      assert.match(await minify(`<div>foo${open}baz${close} bar</div>`), `<div>foo${open}baz${close} bar</div>`);
      assert.match(await minify(`<div>foo ${open} baz ${close} bar</div>`), `<div>foo ${open} baz ${close} bar</div>`);
      assert.match(await minify(`<div>foo${open} baz ${close}bar</div>`), `<div>foo${open} baz ${close}bar</div>`);
      assert.match(await minify(`<div>foo ${open} baz ${close}bar</div>`), `<div>foo ${open} baz ${close}bar</div>`);
      assert.match(await minify(`<div>foo${open} baz ${close} bar</div>`), `<div>foo${open} baz ${close} bar</div>`);
    })
  );
  // Don't trim whitespace around element, but do trim within
  await Promise.all(
    ['bdi', 'bdo', 'button', 'cite', 'code', 'dfn', 'math', 'q', 'rt', 'rtc', 'ruby', 'svg'].map(async (el) => {
      const [open, close] = [`<${el}>`, `</${el}>`];
      assert.match(await minify(`foo ${open}baz${close} bar`), `foo ${open}baz${close} bar`);
      assert.match(await minify(`foo${open}baz${close}bar`), `foo${open}baz${close}bar`);
      assert.match(await minify(`foo ${open}baz${close}bar`), `foo ${open}baz${close}bar`);
      assert.match(await minify(`foo${open}baz${close} bar`), `foo${open}baz${close} bar`);
      assert.match(await minify(`foo ${open} baz ${close} bar`), `foo ${open} baz ${close} bar`);
      assert.match(await minify(`foo${open} baz ${close}bar`), `foo${open} baz ${close}bar`);
      assert.match(await minify(`foo ${open} baz ${close}bar`), `foo ${open} baz ${close}bar`);
      assert.match(await minify(`foo${open} baz ${close} bar`), `foo${open} baz ${close} bar`);
      assert.match(await minify(`<div>foo ${open}baz${close} bar</div>`), `<div>foo ${open}baz${close} bar</div>`);
      assert.match(await minify(`<div>foo${open}baz${close}bar</div>`), `<div>foo${open}baz${close}bar</div>`);
      assert.match(await minify(`<div>foo ${open}baz${close}bar</div>`), `<div>foo ${open}baz${close}bar</div>`);
      assert.match(await minify(`<div>foo${open}baz${close} bar</div>`), `<div>foo${open}baz${close} bar</div>`);
      assert.match(await minify(`<div>foo ${open} baz ${close} bar</div>`), `<div>foo ${open} baz ${close} bar</div>`);
      assert.match(await minify(`<div>foo${open} baz ${close}bar</div>`), `<div>foo${open} baz ${close}bar</div>`);
      assert.match(await minify(`<div>foo ${open} baz ${close}bar</div>`), `<div>foo ${open} baz ${close}bar</div>`);
      assert.match(await minify(`<div>foo${open} baz ${close} bar</div>`), `<div>foo${open} baz ${close} bar</div>`);
    })
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
      assert.match(await minify(input), output);
    })
  );
});

test('surrounded by newlines (astro#7401)', async () => {
  const input = '<span>foo</span>\n\t\tbar\n\t\t<span>baz</span>';
  const output = '<span>foo</span>\nbar\n<span>baz</span>';
  const result = await minify(input);

  assert.match(result, output);
});

test('separated by newlines (#815)', async () => {
  const input = '<p>\n\ta\n\t<span>b</span>\n\tc\n</p>';
  const output = '<p>\na\n<span>b</span>\nc\n</p>';
  const result = await minify(input);

  assert.match(result, output);
});
