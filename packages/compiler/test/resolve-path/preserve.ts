import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `
---
import Foo from './Foo.jsx'
import Bar from './Bar.jsx'
import { name } './foo.module.css'
---
<Foo />
<Foo client:load />
<Foo client:only="react" />
`;

let result: unknown;
test.before(async () => {
	result = await transform(FIXTURE, {
		resolvePath: async (s) => s,
	});
});

test('preserve path', () => {
	assert.match(result.code, /"client:load":true.*"client:component-path":\("\.\/Foo\.jsx"\)/);
	assert.match(result.code, /"client:only":"react".*"client:component-path":\("\.\/Foo\.jsx"\)/);
});

test('no metadata', () => {
	assert.not.match(result.code, /\$\$metadata/);
	assert.not.match(result.code, /\$\$createMetadata/);
	assert.not.match(result.code, /createMetadata as \$\$createMetadata/);
	assert.not.match(result.code, /import \* as \$\$module\d/);
});

test.run();
