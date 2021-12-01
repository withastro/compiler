/* eslint-disable no-console */
import { transform } from '@astrojs/compiler';

const contents = `
---
const { items, emptyItems } = Astro.props;

const internal = [];
---

<!-- False -->
{false && (
  <span id="frag-false" />
)}

<!-- Null -->
{null && (
  <span id="frag-null" />
)}

<!-- True -->
{true && (
  <span id="frag-true" />
)}

<!-- Undefined -->
{false && (<span id="frag-undefined" />)}
`;

async function run() {
  const result = await transform(contents, {
    sourcemap: true,
    as: 'fragment',
    site: undefined,
    sourcefile: 'MoreMenu.astro',
    sourcemap: 'both',
    internalURL: 'astro/internal',
  });

  if (!result.code) {
    throw new Error('Unable to compile top-level expression!');
  }
}

await run();
