import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `{
  node.shouldRenderChildren() ? (
    // IMPORTANT - DO NOT SELF CLOSE THIS TAG. ASTRO FREAKS OUT.
    <Fragment set:html={children}></Fragment>
  ) : node.shouldRenderSelf() ? (
    // @ts-ignore
    content.map((element) => {
      return <Astro.self content={element} components={components} />;
    })
  ) : node.shouldRenderTag() ? (
    <Tag {...props}>
      {node.hasChildren() ? (
        <Astro.self content={children} components={components} />
      ) : null}
    </Tag>
  ) : null
}
`;

let result: unknown;
test.before(async () => {
  result = await transform(FIXTURE);
});

test('does not add trailing newline to rendered output', () => {
  assert.match(result.code, `}\`;\n}, '<stdin>', undefined);`, 'Does not include a trailing newline in the render function');
});

test.run();
