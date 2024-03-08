import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
---
let value = 'world';
---

<Base><body class="foobar"><slot /></body></Base>
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE);
});

test('top-level component does not drop body attributes', () => {
  console.log(result.code);
  assert.match(result.code, "${$$renderComponent($$result,'Base',Base,{},{\"default\": () => $$render`${$$maybeRenderHead($$result)}<body class=\"foobar\">${$$renderSlot($$result,$$slots[\"default\"])}</body>`,})}", `Expected body to be included!`);
});


test.run();
