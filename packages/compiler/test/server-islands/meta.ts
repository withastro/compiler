import assert from 'node:assert/strict';
import { before, describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
---
import Avatar from './Avatar.astro';
import {Other} from './Other.astro';
---

<Avatar server:defer />
<Other server:defer />
`;

let result: Awaited<ReturnType<typeof transform>>;

describe('server-islands/meta', () => {
	before(async () => {
		result = await transform(FIXTURE, {
			resolvePath: (s: string) => {
				const out = new URL(s, import.meta.url);
				return fileURLToPath(out);
			},
		});
	});

	it('component metadata added', () => {
		assert.deepStrictEqual(result.serverComponents.length, 2);
	});

	it('component should contain head propagation', () => {
		assert.deepStrictEqual(result.propagation, true);
	});

	it('path resolved to the filename', () => {
		const m = result.serverComponents[0];
		assert.ok(m.specifier !== m.resolvedPath);
	});

	it('localName is the name used in the template', () => {
		assert.deepStrictEqual(result.serverComponents[0].localName, 'Avatar');
		assert.deepStrictEqual(result.serverComponents[1].localName, 'Other');
	});

	it('exportName is the export name of the imported module', () => {
		assert.deepStrictEqual(result.serverComponents[0].exportName, 'default');
		assert.deepStrictEqual(result.serverComponents[1].exportName, 'Other');
	});
});
