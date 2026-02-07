import { type TransformResult, transform } from '@astrojs/compiler';
import { describe, it, before } from 'node:test';
import assert from 'node:assert/strict';

const FIXTURE = `
---
import Foo from './Foo.jsx'
import Bar from './Bar.jsx'
import { name } from './foo.module.css'
---
<Foo />
<Foo client:load />
<Foo client:only="react" />
`;

let result: TransformResult;

describe('resolve-path/preserve', () => {
	before(async () => {
		result = await transform(FIXTURE, {
			resolvePath: async (s) => s,
		});
	});

	it('preserve path', () => {
		assert.match(result.code, /"client:load":true.*"client:component-path":\("\.\/Foo\.jsx"\)/);
		assert.match(result.code, /"client:only":"react".*"client:component-path":\("\.\/Foo\.jsx"\)/);
	});

	it('no metadata', () => {
		assert.doesNotMatch(result.code, /\$\$metadata/);
		assert.doesNotMatch(result.code, /\$\$createMetadata/);
		assert.doesNotMatch(result.code, /createMetadata as \$\$createMetadata/);
		assert.doesNotMatch(result.code, /import \* as \$\$module\d/);
	});
});
