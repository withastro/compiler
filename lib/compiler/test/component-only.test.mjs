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
import { Markdown } from 'astro/components';
import Layout from '../layouts/content.astro';
---

<style>
  #root {
    color: green;
  }
</style>

<Layout>
  <div id="root">
    <Markdown>
      ## Interesting Topic
    </Markdown>
  </div>
</Layout>`, // NOTE: the lack of trailing space is important to this test!
    {
      sourcemap: true,
      preprocessStyle: async (value, attrs) => {
        if (attrs.lang === 'scss') {
          try {
            return transformSass(value);
          } catch (err) {
            console.error(err);
          }
        }
        return null;
      },
      as: 'document'
    }
  );

  console.log(result.code)

  if (result.code.includes('html')) {
    throw new Error('Result did not remove <html>')
  }
}

await run();
