import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

test('getStaticPaths with braces on newline', async () => {
  const FIXTURE = `---
import A from './A.astro';
export async function getStaticPaths()
{
  return [
    { params: { id: '1' } },
    { params: { id: '2' } },
    { params: { id: '3' } }
  ];
}
---

<div></div>
`;
  const result = await transform(FIXTURE);
  assert.match(result.code, 'export async function getStaticPaths()\n{', 'Expected output to contain getStaticPaths output');
});

test('getStaticPaths as const without braces', async () => {
  const FIXTURE = `---
import A from './A.astro';
export const getStaticPaths = () => ([
  { params: { id: '1' } },
  { params: { id: '2' } },
  { params: { id: '3' } }
])
---

<div></div>
`;
  const result = await transform(FIXTURE);
  assert.match(result.code, 'export const getStaticPaths = () => ([', 'Expected output to contain getStaticPaths output');
});

test('getStaticPaths as const with braces on newline', async () => {
  const FIXTURE = `---
import A from './A.astro';
export const getStaticPaths = () =>
{
  return [
    { params: { id: '1' } },
    { params: { id: '2' } },
    { params: { id: '3' } }
  ];
}
---

<div></div>
`;
  const result = await transform(FIXTURE);
  assert.match(result.code, 'export const getStaticPaths = () =>\n{', 'Expected output to contain getStaticPaths output');
});

test('getStaticPaths with whitespace', async () => {
  const FIXTURE = `---
export const getStaticPaths = async () => {
	const content = await Astro.glob('../content/*.mdx');

	return content
    .filter((page) => !page.frontmatter.draft) // skip drafts
    .map(({ default: MdxContent, frontmatter, url, file }) => {
        return {
          params: { slug: frontmatter.slug || "index" },
          props: {
            MdxContent,
						file,
            frontmatter,
						url
          }
        }
     })
}

const { MdxContent, frontmatter, url, file } = Astro.props;
---
<div></div>
`;
  const result = await transform(FIXTURE);
  assert.match(result.code, '\nconst $$stdin = ', 'Expected getStaticPaths hoisting to maintain newlines');
});

test('getStaticPaths with types', async () => {
  const FIXTURE = `---
export async function getStaticPaths({
  paginate,
}: {
  paginate: PaginateFunction;
}) {
  const allPages = (
    await getCollection(
      "blog"
    )
  );
  return paginate(allPages, { pageSize: 10 });
}
---

<div></div>
`;
  const result = await transform(FIXTURE);
  assert.match(result.code, `{\n  paginate: PaginateFunction;\n}) {`, 'Expected output to contain getStaticPaths output');
});


test.run();
