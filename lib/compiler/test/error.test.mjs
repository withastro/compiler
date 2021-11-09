import { transform } from '@astrojs/compiler';

async function run() {
  try {
    await transform(
      `{metaInfo.map((array) => <dl>
        {Object.entries(array).map((info) => <<<<<<<>>>>>
            <dt>{info[0]}</dt>
            <dd>{info[1]}</dd>
        </>)}
    </dl>)}`,
      {
        as: 'fragment',
        site: undefined,
        sourcefile: `${process.cwd()}/src/pages/namespaced.astro`,
        sourcemap: 'both',
        internalURL: 'astro/internal',
        preprocessStyle: async (value, attrs) => {
          return null;
        },
      }
    );
  } catch (e) {
    if (!e.toString().includes('SyntaxError')) {
      throw e;
    }
  }
}

await run();
