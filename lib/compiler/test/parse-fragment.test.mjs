/* eslint-disable no-console */

import { parse } from '@astrojs/compiler';

const src = `<>Hello</><Fragment>World</Fragment>`;

async function run() {
  const result = await parse(src);

  const [first, second] = result.ast.children;
  if (first.type !== 'fragment') {
    throw new Error(`Expected first child node to be of type "fragment"`);
  }
  if (first.name !== '') {
    throw new Error(`Expected first child node to have name of ""`);
  }
  if (second.type !== 'fragment') {
    throw new Error(`Expected second child node to be of type "fragment"`);
  }
  if (second.name !== 'Fragment') {
    throw new Error(`Expected second child node to have name of "Fragment"`);
  }
}

await run();
