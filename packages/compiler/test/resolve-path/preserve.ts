import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
---
import Foo from './Foo.jsx'
---
<Foo />
<Foo client:load />
<Foo client:only="react" />
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE, {
    resolvePath: async (s) => s,
  });
});

test('preserve path', () => {
  assert.match(result.code, /"client:load":true.*"client:component-path":\("\.\/Foo\.jsx"\)/);
  assert.match(result.code, /"client:only":"react".*"client:component-path":\("\.\/Foo\.jsx"\)/);
});

test.run();
