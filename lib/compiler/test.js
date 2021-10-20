const sleep = (ms) => new Promise((res) => setTimeout(res, ms));

(async () => {
  const { transform } = await import("@astrojs/compiler");
  let i = 0;
  try {
    const value = await transform(
      `---
let value = 'world';
---

<style lang="scss" define:vars={{ a: 0 }}>
div {
  color: red;
}
</style>

<div>Hello world!</div>

<div>Ahhh</div>

<style lang="scss">
div {
  color: green;
}
</style>
`,
      {
        sourcemap: true,
        // HOLY CRAP THIS ACTUALLY WORKS!
        preprocessStyle: async (value, attrs) => {
          let x = i++;
          if (!attrs.lang) {
            return null;
          }
          console.log(`Starting to preprocess style ${x} as ${attrs.lang}`);
          await sleep(3000);
          console.log(`Finished preprocessing ${x}`);
          return value.replace('color', 'background');
        },
      }
    );
    console.log(value)
  } catch (e) {
    console.log(e);
  }
  // const start = performance.now()
  // const html = await compile(template);
  // const end = performance.now()

  // console.log('Compiled in ' + (start - end).toFixed(1) + 'ms');
  // console.log(html);
})();
