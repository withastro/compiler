import { fileURLToPath } from 'node:url';
import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `
---
import Avatar from './Avatar.astro';
import {Other} from './Other.astro';
---

<Avatar server:defer />
<Other server:defer />
`;

let result: Awaited<ReturnType<typeof transform>>;
test.before(async () => {
	result = await transform(FIXTURE, {
		resolvePath: async (s: string) => {
			const out = new URL(s, import.meta.url);
			return fileURLToPath(out);
		},
	});
});

test('component metadata added', () => {
	assert.equal(result.serverComponents.length, 2);
});

test('path resolved to the filename', () => {
	const m = result.serverComponents[0];
	assert.ok(m.specifier !== m.resolvedPath);
});

test('localName is the name used in the template', () => {
	assert.equal(result.serverComponents[0].localName, 'Avatar');
	assert.equal(result.serverComponents[1].localName, 'Other');
});

test('exportName is the export name of the imported module', () => {
	assert.equal(result.serverComponents[0].exportName, 'default');
	assert.equal(result.serverComponents[1].exportName, 'Other');
});

test.run();
