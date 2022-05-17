import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { convertToTSX } from '@astrojs/compiler';

const FIXTURE = `
---
let value = 'world';
---

<h1 name="value" empty {shorthand} expression={true} literal=\`tags\`>Hello {value}</h1>
<div></div>
`;

const EXPECTED = `export default async (props) => {
let value = 'world';
return (<Fragment>
<h1 name="value" empty shorthand={shorthand} expression={true} literal={\`tags\`}>Hello {value}</h1>
<div></div>
</Fragment>);
}
`;

let result;
test.before(async () => {
  try {
    result = await convertToTSX(FIXTURE);
  } catch (e) {
    console.log(e);
  }
});

test('returns', () => {
  assert.type(result, 'object', `Expected "convertToTSX" to return an object!`);
});

test('output', () => {
  assert.snapshot(result.code, EXPECTED, `expected code to match snapshot`);
});

test.run();
