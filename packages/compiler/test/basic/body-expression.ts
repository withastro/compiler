import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `
---
const slugs = ['one', 'two', 'three'];
---

<html>
  <head></head>
  <body>
    {slugs.map((slug) => (
      <a href={\`/post/\${slug}\`}>{slug}</a>
    ))}
  </body>
</html>
`;

let result: TransformResult;
test.before(async () => {
	result = await transform(FIXTURE);
});

test('can compiler body expression', () => {
	assert.ok(result.code, 'Expected to compiler body expression!');
});

test.run();
