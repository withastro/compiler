import { type TransformResult, transform } from '@astrojs/compiler-rs';
import { before, describe, it } from 'node:test';
import assert from 'node:assert/strict';

const FIXTURE = `
---
import CaretCounter from '../components/^--with-carets/Counter';
import RocketCounter from '../components/and-rockets-🚀/Counter';
import PercentCounter from '../components/now-100%-better/Counter';
import SpaceCounter from '../components/with some spaces/Counter';
import RoundBracketCounter from '../components/with-(round-brackets)/Counter';
import SquareBracketCounter from '../components/with-[square-brackets]/Counter';
import RemoteComponent from 'https://test.com/components/with-[wacky-brackets}()10%-cooler/Counter';
---
<html>
<body>
  <h1>Special chars in component import paths from an .astro file</h1>
  <CaretCounter id="caret" client:visible />
  <RocketCounter id="rocket" client:visible />
  <PercentCounter id="percent" client:visible />
  <SpaceCounter id="space" client:visible />
  <RoundBracketCounter id="round-bracket" client:visible />
  <SquareBracketCounter id="square-bracket" client:visible />
  <RemoteComponent id="remote-component" client:visible />
</body>
</html>
`;

describe('client-directive/special-characters', () => {
	let result: TransformResult;
	before(async () => {
		result = await transform(FIXTURE, {
			filename: '/users/astro/apps/pacman/src/pages/index.astro',
		});
	});

	it('does not panic', () => {
		assert.ok(result.code);
	});

	it('hydrated components with carets', () => {
		const components = result.hydratedComponents;
		assert.deepStrictEqual(components[0].exportName, 'default');
		assert.deepStrictEqual(components[0].specifier, '../components/^--with-carets/Counter');
		assert.deepStrictEqual(
			components[0].resolvedPath,
			'/users/astro/apps/pacman/src/components/^--with-carets/Counter',
		);
	});

	it('hydrated components with rockets', () => {
		const components = result.hydratedComponents;
		assert.deepStrictEqual(components[1].exportName, 'default');
		assert.deepStrictEqual(components[1].specifier, '../components/and-rockets-🚀/Counter');
		assert.deepStrictEqual(
			components[1].resolvedPath,
			'/users/astro/apps/pacman/src/components/and-rockets-🚀/Counter',
		);
	});

	it('hydrated components with percent', () => {
		const components = result.hydratedComponents;
		assert.deepStrictEqual(components[2].exportName, 'default');
		assert.deepStrictEqual(components[2].specifier, '../components/now-100%-better/Counter');
		assert.deepStrictEqual(
			components[2].resolvedPath,
			'/users/astro/apps/pacman/src/components/now-100%-better/Counter',
		);
	});

	it('hydrated components with spaces', () => {
		const components = result.hydratedComponents;
		assert.deepStrictEqual(components[3].exportName, 'default');
		assert.deepStrictEqual(components[3].specifier, '../components/with some spaces/Counter');
		assert.deepStrictEqual(
			components[3].resolvedPath,
			'/users/astro/apps/pacman/src/components/with some spaces/Counter',
		);
	});

	it('hydrated components with round brackets', () => {
		const components = result.hydratedComponents;
		assert.deepStrictEqual(components[4].exportName, 'default');
		assert.deepStrictEqual(components[4].specifier, '../components/with-(round-brackets)/Counter');
		assert.deepStrictEqual(
			components[4].resolvedPath,
			'/users/astro/apps/pacman/src/components/with-(round-brackets)/Counter',
		);
	});

	it('hydrated components with square brackets', () => {
		const components = result.hydratedComponents;
		assert.deepStrictEqual(components[5].exportName, 'default');
		assert.deepStrictEqual(components[5].specifier, '../components/with-[square-brackets]/Counter');
		assert.deepStrictEqual(
			components[5].resolvedPath,
			'/users/astro/apps/pacman/src/components/with-[square-brackets]/Counter',
		);
	});

	it('hydrated components with kitchen-sink', () => {
		const components = result.hydratedComponents;
		assert.deepStrictEqual(components[6].exportName, 'default');
		assert.deepStrictEqual(
			components[6].specifier,
			'https://test.com/components/with-[wacky-brackets}()10%-cooler/Counter',
		);
		assert.deepStrictEqual(
			components[6].resolvedPath,
			'https://test.com/components/with-[wacky-brackets}()10%-cooler/Counter',
		);
	});
});
