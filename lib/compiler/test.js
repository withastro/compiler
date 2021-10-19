(async () => {
  const { init, transform } = await import('@astrojs/compiler');

  await init({
    // NOTE: this can't be async yet...
    stylePreprocess: (value, lang) => {
      return 'test {}';
    }
  })

  try {
    const value = await transform(`---
let value = 'world';
---

<style lang="scss">
div {
  color: red;
}
</style>
`, { sourcemap: 'both' });
  console.log(performance.now(), value);
  } catch (e) {
    console.log(e)
  }
  // const start = performance.now()
  // const html = await compile(template);
  // const end = performance.now()

  // console.log('Compiled in ' + (start - end).toFixed(1) + 'ms');
  // console.log(html);
})();
