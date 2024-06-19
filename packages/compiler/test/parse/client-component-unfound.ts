import { parse } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `{
	headers && (
		<nav class="mobile-toc">
			<TableOfContents
				client:media="(max-width: 72em)"
				headers={headers}
				labels={{ onThisPage: t('rightSidebar.onThisPage'), overview: t('rightSidebar.overview') }}
				isMobile={true}
			/>
		</nav>
	)
}
`;

test('unfound client component', async () => {
  const result = await parse(FIXTURE);
  assert.ok(result.ast, 'Expected an AST to be generated');
});

test.run();
