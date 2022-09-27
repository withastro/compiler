import fs from 'node:fs/promises';
import { convertToTSX } from '@astrojs/compiler';

const SRC = `---
const data = await fetch('whatever')
---

<script>
  function hello(params) {}

  hello({});
</script>

<div />
<div hello="world" />
<div a="1"></div>
`;


console.log(await convertToTSX(SRC, { sourcemap: 'inline', sourcefile: 'test.astro' }).then((res) => res.code));
