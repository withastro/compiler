/* eslint-disable no-console */

import { transform } from '@astrojs/compiler';

async function run() {
  const result = await transform(
    `---
    import ThemeToggleButton from './ThemeToggleButton.tsx';
    ---
    <style>
      body {
        background: blue;
      }
    </style>
    <div>
      <ThemeToggleButton client:visible />
    </div>`,
    {
      sourcemap: true,
      as: 'fragment',
      site: undefined,
      sourcefile: 'MoreMenu.astro',
      sourcemap: 'both',
      internalURL: 'astro/internal',
      preprocessStyle: async (value, attrs) => {
        return null;
      },
    }
  );

  // test
  if (!result.code.includes(`{ modules: [{ module: $$module1, specifier: './ThemeToggleButton.tsx' }], hydratedComponents: [ThemeToggleButton], hoisted: [] }`)) {
    throw new Error('Hydrated components not included');
  }
}

await run();
