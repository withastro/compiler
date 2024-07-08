import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

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

let result: unknown;
test.before(async () => {
  result = await transform(FIXTURE, { filename: '/users/astro/apps/pacman/src/pages/index.astro' });
});

test('does not panic', () => {
  assert.ok(result.code);
});

test('hydrated components with carets', () => {
  const components = result.hydratedComponents;
  assert.equal(components[0].exportName, 'default');
  assert.equal(components[0].specifier, '../components/^--with-carets/Counter');
  assert.equal(components[0].resolvedPath, '/users/astro/apps/pacman/src/components/^--with-carets/Counter');
});

test('hydrated components with rockets', () => {
  const components = result.hydratedComponents;
  assert.equal(components[1].exportName, 'default');
  assert.equal(components[1].specifier, '../components/and-rockets-🚀/Counter');
  assert.equal(components[1].resolvedPath, '/users/astro/apps/pacman/src/components/and-rockets-🚀/Counter');
});

test('hydrated components with percent', () => {
  const components = result.hydratedComponents;
  assert.equal(components[2].exportName, 'default');
  assert.equal(components[2].specifier, '../components/now-100%-better/Counter');
  assert.equal(components[2].resolvedPath, '/users/astro/apps/pacman/src/components/now-100%-better/Counter');
});

test('hydrated components with spaces', () => {
  const components = result.hydratedComponents;
  assert.equal(components[3].exportName, 'default');
  assert.equal(components[3].specifier, '../components/with some spaces/Counter');
  assert.equal(components[3].resolvedPath, '/users/astro/apps/pacman/src/components/with some spaces/Counter');
});

test('hydrated components with round brackets', () => {
  const components = result.hydratedComponents;
  assert.equal(components[4].exportName, 'default');
  assert.equal(components[4].specifier, '../components/with-(round-brackets)/Counter');
  assert.equal(components[4].resolvedPath, '/users/astro/apps/pacman/src/components/with-(round-brackets)/Counter');
});

test('hydrated components with square brackets', () => {
  const components = result.hydratedComponents;
  assert.equal(components[5].exportName, 'default');
  assert.equal(components[5].specifier, '../components/with-[square-brackets]/Counter');
  assert.equal(components[5].resolvedPath, '/users/astro/apps/pacman/src/components/with-[square-brackets]/Counter');
});

test('hydrated components with kitchen-sink', () => {
  const components = result.hydratedComponents;
  assert.equal(components[6].exportName, 'default');
  assert.equal(components[6].specifier, 'https://test.com/components/with-[wacky-brackets}()10%-cooler/Counter');
  assert.equal(components[6].resolvedPath, 'https://test.com/components/with-[wacky-brackets}()10%-cooler/Counter');
});

test.run();
