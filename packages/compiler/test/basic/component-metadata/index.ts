import { type TransformResult, transform } from '@astrojs/compiler';
import { before, describe, it } from 'node:test';
import assert from 'node:assert/strict';

const FIXTURE = `
---
import One from '../components/one.jsx';
import * as Two from '../components/two.jsx';
import { Three } from '../components/three.tsx';
import * as four from '../components/four.jsx';

import * as Five from '../components/five.jsx';
import { Six } from '../components/six.jsx';
import Seven from '../components/seven.jsx';
import * as eight from '../components/eight.jsx';
---

<One client:load />
<Two.someName client:load />
<Three client:load />
<four.nested.deep.Component client:load />

<!-- client only tests -->
<Five.someName client:only />
<Six client:only />
<Seven client:only />
<eight.nested.deep.Component client:only />
`;

describe('component-metadata', () => {
	let result: TransformResult;
	before(async () => {
		result = await transform(FIXTURE, {
			filename: '/users/astro/apps/pacman/src/pages/index.astro',
		});
	});

	it('Hydrated component', () => {
		const components = result.hydratedComponents;
		assert.deepStrictEqual(components.length, 4);
	});

	it('Hydrated components: default export', () => {
		const components = result.hydratedComponents;
		assert.deepStrictEqual(components[0].exportName, 'default');
		assert.deepStrictEqual(components[0].specifier, '../components/one.jsx');
		assert.deepStrictEqual(components[0].resolvedPath, '/users/astro/apps/pacman/src/components/one.jsx');
	});

	it('Hydrated components: star export', () => {
		const components = result.hydratedComponents;
		assert.deepStrictEqual(components[1].exportName, 'someName');
		assert.deepStrictEqual(components[1].specifier, '../components/two.jsx');
		assert.deepStrictEqual(components[1].resolvedPath, '/users/astro/apps/pacman/src/components/two.jsx');
	});

	it('Hydrated components: named export', () => {
		const components = result.hydratedComponents;
		assert.deepStrictEqual(components[2].exportName, 'Three');
		assert.deepStrictEqual(components[2].specifier, '../components/three.tsx');
		assert.deepStrictEqual(components[2].resolvedPath, '/users/astro/apps/pacman/src/components/three.tsx');
	});

	it('Hydrated components: deep nested export', () => {
		const components = result.hydratedComponents;
		assert.deepStrictEqual(components[3].exportName, 'nested.deep.Component');
		assert.deepStrictEqual(components[3].specifier, '../components/four.jsx');
		assert.deepStrictEqual(components[3].resolvedPath, '/users/astro/apps/pacman/src/components/four.jsx');
	});

	it('ClientOnly component', () => {
		const components = result.clientOnlyComponents;
		assert.deepStrictEqual(components.length, 4);
	});

	it('ClientOnly components: star export', () => {
		const components = result.clientOnlyComponents;
		assert.deepStrictEqual(components[0].exportName, 'someName');
		assert.deepStrictEqual(components[0].specifier, '../components/five.jsx');
		assert.deepStrictEqual(components[0].resolvedPath, '/users/astro/apps/pacman/src/components/five.jsx');
	});

	it('ClientOnly components: named export', () => {
		const components = result.clientOnlyComponents;
		assert.deepStrictEqual(components[1].exportName, 'Six');
		assert.deepStrictEqual(components[1].specifier, '../components/six.jsx');
		assert.deepStrictEqual(components[1].resolvedPath, '/users/astro/apps/pacman/src/components/six.jsx');
	});

	it('ClientOnly components: default export', () => {
		const components = result.clientOnlyComponents;
		assert.deepStrictEqual(components[2].exportName, 'default');
		assert.deepStrictEqual(components[2].specifier, '../components/seven.jsx');
		assert.deepStrictEqual(components[2].resolvedPath, '/users/astro/apps/pacman/src/components/seven.jsx');
	});

	it('ClientOnly components: deep nested export', () => {
		const components = result.clientOnlyComponents;
		assert.deepStrictEqual(components[3].exportName, 'nested.deep.Component');
		assert.deepStrictEqual(components[3].specifier, '../components/eight.jsx');
		assert.deepStrictEqual(components[3].resolvedPath, '/users/astro/apps/pacman/src/components/eight.jsx');
	});
});
