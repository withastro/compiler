import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    < data-test="hello"><div></div></>
  </body>
</html>`;

let result: unknown;
test.before(async () => {
	result = await convertToTSX(FIXTURE, {
		filename: '/src/components/fragment.astro',
	});
});

test('got a tokenizer error', () => {
	assert.ok(Array.isArray(result.diagnostics));
	assert.is(result.diagnostics.length, 1);
	assert.is(
		result.diagnostics[0].text,
		'Unable to assign attributes when using <> Fragment shorthand syntax!'
	);
	const loc = result.diagnostics[0].location;
	assert.is(FIXTURE.split('\n')[loc.line - 1], `    < data-test="hello"><div></div></>`);
	assert.is(
		FIXTURE.split('\n')[loc.line - 1].slice(loc.column - 1, loc.column - 1 + loc.length),
		`< data-test="hello">`
	);
});

test.run();
