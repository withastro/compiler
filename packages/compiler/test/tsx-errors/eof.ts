import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `<html>
  <head>
    <title>Hello world</title>
  </head>
  <body>
    <div>
      {/*
    </div>
  </body>
</html>`;

let result: TransformResult;
test.before(async () => {
	result = await convertToTSX(FIXTURE, {
		filename: '/src/components/EOF.astro',
	});
});

test('got a tokenizer error', () => {
	assert.ok(Array.isArray(result.diagnostics));
	assert.is(result.diagnostics.length, 1);
	assert.is(result.diagnostics[0].text, 'Unterminated comment');
	assert.is(FIXTURE.split('\n')[result.diagnostics[0].location.line - 1], '      {/*');
});

test.run();
