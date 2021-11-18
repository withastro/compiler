/* eslint-disable no-console */

import { transform } from '@astrojs/compiler';
import sass from 'sass';

function transformSass(value) {
  return new Promise((resolve, reject) => {
    sass.render({ data: value }, (err, result) => {
      if (err) {
        reject(err);
        return;
      }
      resolve({ code: result.css.toString('utf8'), map: result.map });
      return;
    });
  });
}

async function run() {
  const result = await transform(
    `---
let value = 'world';
---

<style lang="scss" define:vars={{ a: 0 }}>
$color: red;

div {
  color: $color;
}
</style>

<div>Hello world!</div>

<div>Ahhh</div>

<style lang="scss">
$color: green;
div {
  color: $color;
}
</style>
`,
    {
      sourcemap: true,
      preprocessStyle: async (value, attrs) => {
        if (!attrs.lang) {
          return null;
        }
        if (attrs.lang === 'scss') {
          try {
            return transformSass(value);
          } catch (err) {
            console.error(err);
          }
        }
        return null;
      },
    }
  );

  // test
  if (!result.code.includes('color:red')) {
    throw new Error(`Styles didn’t transform as expected. Expected "color:red" to be present.`);
  }

  if (!result.code.includes('color:green')) {
    throw new Error(`Styles didn’t transform as expected. Expected "color:green" to be present.`);
  }
}

await run();
