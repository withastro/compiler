import { type ParseResult, parse } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

test('can compile unfinished style', async () => {
	let error = 0;
	let result: ParseResult;

	try {
		result = await parse('<style>');
	} catch (e) {
		error = 1;
	}

	const style = result.ast.children[0];
	assert.equal(error, 0, 'Expected to compile with unfinished style.');
	assert.ok(result.ast, 'Expected to compile with unfinished style.');
	assert.equal(style.name, 'style', 'Expected to compile with unfinished style.');
});

test.run();
