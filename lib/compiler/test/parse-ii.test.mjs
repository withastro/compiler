/* eslint-disable no-console */

import { parse } from '@astrojs/compiler';
import { walk, is } from '@astrojs/compiler/utils';

const src = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta http-equiv="X-UA-Compatible" content="IE=edge">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Document</title>
</head>
  <body>
    <h1>
      Hello world!</h1>
  </body>
</html>
`;

async function run() {
  const result = await parse(src);

  const [doctype, html, ...others] = result.ast.children;
  if (others.length > 0) {
    throw new Error(`Expected only two child nodes!`);
  }
  if (doctype.type !== 'doctype') {
    throw new Error(`Expected first child node to be of type "doctype"`);
  }
  if (html.type !== 'element') {
    throw new Error(`Expected second child node to be of type "element"`);
  }
}

await run();
