/* eslint-disable no-console */
import { transform } from '@astrojs/compiler';

const contents = `---
const slugs = ['one', 'two', 'three'];
---

<html>
  <head>
  </head>
  <body>
    {slugs.map((slug) => (
      <a href={\`/post/\${slug}\`}>{slug}</a>
    ))}
  </body>
</html>
`;

async function run() {
  const result = await transform(contents, {
    sourcemap: true,
    as: 'document',
    site: undefined,
    sourcefile: 'MoreMenu.astro',
    sourcemap: 'both',
    internalURL: 'astro/internal',
  });

  console.log(result.code);

  if (!result.code) {
    throw new Error('Unable to compile body expression!');
  }
}

await run();
