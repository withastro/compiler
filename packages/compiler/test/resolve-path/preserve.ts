import assert from 'node:assert/strict';
import { before, describe, it } from 'node:test';
import { type TransformResult, transform } from '@astrojs/compiler-rs';
import { transform as transformAsync } from '@astrojs/compiler-rs/async';

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
	before(() => {
		result = transform(FIXTURE, {
			resolvePath: (s) => s,
		});
	});

	it('preserve path', () => {
		assert.match(result.code, /"client:load": true[\s\S]*"client:component-path": "\.\/Foo\.jsx"/);
		assert.match(
			result.code,
			/"client:only": "react"[\s\S]*"client:component-path": "\.\/Foo\.jsx"/,
		);
	});

	it('no metadata', () => {
		assert.doesNotMatch(result.code, /\$\$metadata/);
		assert.doesNotMatch(result.code, /\$\$createMetadata/);
		assert.doesNotMatch(result.code, /createMetadata as \$\$createMetadata/);
		assert.doesNotMatch(result.code, /import \* as \$\$module\d/);
	});

	it('resolvePath rewrites code string (async)', async () => {
		const resolved = await transformAsync(FIXTURE, {
			resolvePath: async (s) => `/resolved${s.slice(1)}`,
		});
		assert.match(resolved.code, /"client:component-path": "\/resolved\/Foo\.jsx"/);
		assert.doesNotMatch(resolved.code, /"client:component-path": "\.\/Foo\.jsx"/);
	});

	it('resolvePath rewrites code string (sync)', () => {
		const resolved = transform(FIXTURE, {
			resolvePath: (s) => `/resolved${s.slice(1)}`,
		});
		assert.match(resolved.code, /"client:component-path": "\/resolved\/Foo\.jsx"/);
		assert.doesNotMatch(resolved.code, /"client:component-path": "\.\/Foo\.jsx"/);
	});
});
