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

<style lang="scss"></style>

<div>Hello world!</div>

<div>Ahhh</div>
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
}

await run();
