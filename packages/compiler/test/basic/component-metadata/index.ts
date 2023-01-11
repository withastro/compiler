import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform, TransformResult } from '@astrojs/compiler';

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

let result: TransformResult;
test.before(async () => {
  result = await transform(FIXTURE, {
    filename: '/users/astro/apps/pacman/src/pages/index.astro',
  });
});

test('Hydrated component', () => {
  let components = result.hydratedComponents;
  assert.equal(components.length, 4);
});

test('Hydrated components: default export', () => {
  let components = result.hydratedComponents;
  assert.equal(components[0].exportName, 'default');
  assert.equal(components[0].specifier, '../components/one.jsx');
  assert.equal(components[0].resolvedPath, '/users/astro/apps/pacman/src/components/one.jsx');
});

test('Hydrated components: star export', () => {
  let components = result.hydratedComponents;
  assert.equal(components[1].exportName, 'someName');
  assert.equal(components[1].specifier, '../components/two.jsx');
  assert.equal(components[1].resolvedPath, '/users/astro/apps/pacman/src/components/two.jsx');
});

test('Hydrated components: named export', () => {
  let components = result.hydratedComponents;
  assert.equal(components[2].exportName, 'Three');
  assert.equal(components[2].specifier, '../components/three.tsx');
  assert.equal(components[2].resolvedPath, '/users/astro/apps/pacman/src/components/three.tsx');
});

test('Hydrated components: deep nested export', () => {
  let components = result.hydratedComponents;
  assert.equal(components[3].exportName, 'nested.deep.Component');
  assert.equal(components[3].specifier, '../components/four.jsx');
  assert.equal(components[3].resolvedPath, '/users/astro/apps/pacman/src/components/four.jsx');
});

test('ClientOnly component', () => {
  let components = result.clientOnlyComponents;
  assert.equal(components.length, 4);
});

test('ClientOnly components: star export', () => {
  let components = result.clientOnlyComponents;
  assert.equal(components[0].exportName, 'someName');
  assert.equal(components[0].specifier, '../components/five.jsx');
  assert.equal(components[0].resolvedPath, '/users/astro/apps/pacman/src/components/five.jsx');
});

test('ClientOnly components: named export', () => {
  let components = result.clientOnlyComponents;
  assert.equal(components[1].exportName, 'Six');
  assert.equal(components[1].specifier, '../components/six.jsx');
  assert.equal(components[1].resolvedPath, '/users/astro/apps/pacman/src/components/six.jsx');
});

test('ClientOnly components: default export', () => {
  let components = result.clientOnlyComponents;
  assert.equal(components[2].exportName, 'default');
  assert.equal(components[2].specifier, '../components/seven.jsx');
  assert.equal(components[2].resolvedPath, '/users/astro/apps/pacman/src/components/seven.jsx');
});

test('ClientOnly components: deep nested export', () => {
  let components = result.clientOnlyComponents;
  assert.equal(components[3].exportName, 'nested.deep.Component');
  assert.equal(components[3].specifier, '../components/eight.jsx');
  assert.equal(components[3].resolvedPath, '/users/astro/apps/pacman/src/components/eight.jsx');
});

test.run();
