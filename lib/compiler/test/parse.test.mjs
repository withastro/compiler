/* eslint-disable no-console */

import { parse } from '@astrojs/compiler';
import { walk, is } from '@astrojs/compiler/utils';

const src = `---
let value = 'world';
---

<h1 name="value" empty {shorthand} expression={true} literal=\`tags\` client:load>Hello {value}</h1>
`;

async function run() {
  const result = await parse(src);

  if (typeof result !== 'object') {
    throw new Error(`Expected "parse" to return an object!`);
  }
  if (result.ast.type !== 'root') {
    throw new Error(`Expected "ast" root node to be of type "root"`);
  }
  const [frontmatter, _, element] = result.ast.children;
  if (frontmatter.type !== 'frontmatter') {
    throw new Error(`Expected first child node to be of type "frontmatter"`);
  }
  if (element.type !== 'element') {
    throw new Error(`Expected third child node to be of type "element"`);
  }

  walk(result.ast, (node) => {
    if (is.tag(node)) {
      if (node.name !== 'h1') {
        throw new Error(`Expected element to be "<h1>"`);
      }
      if (!node.directives || (Array.isArray(node.directives) && !(node.directives.length >= 1))) {
        throw new Error(`Expected client:load directive to be on "<h1>"`);
      }
    }
  });
}

await run();
