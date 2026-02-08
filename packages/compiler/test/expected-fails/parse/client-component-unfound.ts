import { parse } from '@astrojs/compiler';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

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

describe('parse/client-component-unfound', { skip: true }, () => {
	it('unfound client component', async () => {
		const result = await parse(FIXTURE);
		assert.ok(result.ast, 'Expected an AST to be generated');
	});
});
