import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

async function minify(input: string) {
  const code = (await transform(input, { compact: true })).code;
  return code;
}

test('basic', async () => {
  assert.match(await minify(`    <div>Hello {value}</div>      `), '$$render`<div>Hello ${value}</div> `');
  assert.match(await minify(`    <div> Hello {value} </div>      `), '$$render`<div> Hello ${value} </div> `');
});

test('preservation', async () => {
  assert.match(await minify(`<pre>  !  </pre>`), '$$render`<pre>  !  </pre>`');
  assert.ok(await minify(`<div is:raw>  !  </div>`), '$$render`<div>  !  </div>`');
  assert.ok(await minify(`<Markdown>  !  </Markdown>`), '$$render`  !  `');
});

test('collapsing', async () => {
  assert.ok(await minify(`<span> inline </span>`), '$$render`<span> inline </span>`');
  assert.ok(await minify(`<span>\n inline \t{\t expression \t}</span>`), '$$render`<span> inline ${ expression } </span>`');
  assert.ok(await minify(`<span> inline { expression }</span>`), '$$render`<span> inline ${ expression }</span>`');
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
  // await Promise.all([
  //   ' a <? b ?> c ',
  //   '<!-- d --> a <? b ?> c ',
  //   ' <!-- d -->a <? b ?> c ',
  //   ' a<!-- d --> <? b ?> c ',
  //   ' a <!-- d --><? b ?> c ',
  //   ' a <? b ?><!-- d --> c ',
  //   ' a <? b ?> <!-- d -->c ',
  //   ' a <? b ?> c<!-- d --> ',
  //   ' a <? b ?> c <!-- d -->'
  // ].map(async (input) => {
  //   expect(await minify(input, {
  //     collapseWhitespace: true,
  //     conservativeCollapse: true
  //   })).toBe(input, input);
  //   expect(await minify(input, {
  //     collapseWhitespace: true,
  //     removeComments: true
  //   })).toBe('a <? b ?> c', input);
  //   expect(await minify(input, {
  //     collapseWhitespace: true,
  //     conservativeCollapse: true,
  //     removeComments: true
  //   })).toBe(' a <? b ?> c ', input);
  //   input = '<p>' + input + '</p>';
  //   expect(await minify(input, {
  //     collapseWhitespace: true,
  //     conservativeCollapse: true
  //   })).toBe(input, input);
  //   expect(await minify(input, {
  //     collapseWhitespace: true,
  //     removeComments: true
  //   })).toBe('<p>a <? b ?> c</p>', input);
  //   expect(await minify(input, {
  //     collapseWhitespace: true,
  //     conservativeCollapse: true,
  //     removeComments: true
  //   })).toBe('<p> a <? b ?> c </p>', input);
  // }));
  // input = '<li><i></i> <b></b> foo</li>';
  // output = '<li><i></i> <b></b> foo</li>';
  // expect(await minify(input, { collapseWhitespace: true })).toBe(output);
  // input = '<li><i> </i> <b></b> foo</li>';
  // expect(await minify(input, { collapseWhitespace: true })).toBe(output);
  // input = '<li> <i></i> <b></b> foo</li>';
  // expect(await minify(input, { collapseWhitespace: true })).toBe(output);
  // input = '<li><i></i> <b> </b> foo</li>';
  // expect(await minify(input, { collapseWhitespace: true })).toBe(output);
  // input = '<li> <i> </i> <b> </b> foo</li>';
  // expect(await minify(input, { collapseWhitespace: true })).toBe(output);
  // input = '<div> <a href="#"> <span> <b> foo </b> <i> bar </i> </span> </a> </div>';
  // output = '<div><a href="#"><span><b>foo </b><i>bar</i></span></a></div>';
  // expect(await minify(input, { collapseWhitespace: true })).toBe(output);
  // input = '<head> <!-- a --> <!-- b --><link> </head>';
  // output = '<head><!-- a --><!-- b --><link></head>';
  // expect(await minify(input, { collapseWhitespace: true })).toBe(output);
  // input = '<head> <!-- a --> <!-- b --> <!-- c --><link> </head>';
  // output = '<head><!-- a --><!-- b --><!-- c --><link></head>';
  // expect(await minify(input, { collapseWhitespace: true })).toBe(output);
  // input = '<p> foo\u00A0bar\nbaz  \u00A0\nmoo\t</p>';
  // output = '<p>foo\u00A0bar baz \u00A0 moo</p>';
  // expect(await minify(input, { collapseWhitespace: true })).toBe(output);
  // input = '<label> foo </label>\n' +
  //   '<input>\n' +
  //   '<object> bar </object>\n' +
  //   '<select> baz </select>\n' +
  //   '<textarea> moo </textarea>\n';
  // output = '<label>foo</label> <input> <object>bar</object> <select>baz</select> <textarea> moo </textarea>';
  // expect(await minify(input, { collapseWhitespace: true })).toBe(output);
  // input = '<pre>\n' +
  //   'foo\n' +
  //   '<br>\n' +
  //   'bar\n' +
  //   '</pre>\n' +
  //   'baz\n';
  // output = '<pre>\nfoo\n<br>\nbar\n</pre>baz';
  // expect(await minify(input, { collapseWhitespace: true })).toBe(output);
});
