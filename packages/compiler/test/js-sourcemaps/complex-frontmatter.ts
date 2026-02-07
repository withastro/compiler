import { describe, it } from 'node:test';
import assert from 'node:assert/strict';
import { testJSSourcemap } from '../utils.js';

const input = `---
// TODO: Due to this issue: https://github.com/withastro/astro/issues/1438, this route can't be in the same folder
// as the paginated article list is or they'll conflict, so this means our articles URL are \`/article\/\${slug}\` instead
// of \`/articles/\${slug}\` (with a s), once that issue is fixed, we'll be able to put it back in the right place

const foobar = true;

import { Article, postProcessArticle } from "$data/articles";
import type { GetStaticPaths, MDXInstance } from "$data/shared";
import ArticleLayout from "$layouts/ArticleLayout.astro";
import { getSlugFromFile } from "$utils";

export const getStaticPaths: GetStaticPaths = async () => {
  const articles = await Astro.glob<Article>("/content/articles/**/*.mdx");
  return articles.map((article) => {
    const augmentedFrontmatter = postProcessArticle(article.frontmatter, article.file);

    return {
      params: { slug: getSlugFromFile(article.file) },
      props: { article: { ...article, frontmatter: augmentedFrontmatter } },
    };
  });
};

interface Props {
  article: MDXInstance<Article>;
}

const { article } = Astro.props;
---

<ArticleLayout article={article} />`;

describe('js-sourcemaps/complex-frontmatter', { skip: true }, () => {
	it('tracks getStaticPaths', async () => {
		const loc = await testJSSourcemap(input, 'getStaticPaths');
		assert.deepStrictEqual(loc, { source: 'index.astro', line: 13, column: 14, name: null });
	});

	it('tracks foobar', async () => {
		const loc = await testJSSourcemap(input, 'foobar');
		assert.deepStrictEqual(loc, { source: 'index.astro', line: 6, column: 7, name: null });
	});
});
