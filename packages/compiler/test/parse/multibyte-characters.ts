import { parse } from '@astrojs/compiler';
import { describe, it } from 'node:test';
import assert from 'node:assert/strict';

const FIXTURE = '{foo}，';

describe('parse/multibyte-characters', { skip: true }, () => {
	it('does not crash', async () => {
		const result = await parse(FIXTURE);
		assert.ok(result.ast, 'does not crash');
	});

	it('properly maps the position', async () => {
		const {
			ast: { children },
		} = await parse(FIXTURE);

		const text = children[1];
		assert.deepStrictEqual(text.position?.start.offset, 5, 'properly maps the text start position');
		assert.deepStrictEqual(text.position?.end?.offset, 8, 'properly maps the text end position');
	});
});
