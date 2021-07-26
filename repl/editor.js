const defaultValue = `---
const text = "Hello world!";
const items = [0, 1, 2];
---

<html>
  <head>
    <title>Astro</title>
  </head>
  <body>
    <div>
      {text}
    </div>
  </body>
</html>
`;

const editor = CodeMirror(document.querySelector('#editor'), {
  tabSize: 2,
  lineNumbers: true,
  value: defaultValue,
  mode: "text/html",
  theme: 'material'
});

editor.setSize("100%", "100%");

editor.on('changes', async () => {
    const value = editor.getValue();
    if (globalThis.BuildPage) {
        await renderResult(value);
    }
});

setTimeout(() => {
  renderResult(defaultValue);
}, 250)

const out = document.querySelector('#output');

async function renderResult(source) {
  console.clear();
  source = source.trim();
  const start = performance.now();
  let output = await globalThis.BuildPage(source);
  const endCompile = performance.now();
  console.log(`Compiled in ${Math.floor(endCompile - start)}ms`);
  let prelude = ['window'].map(key => `const ${key} = undefined;`).join('');
  const dataUri = 'data:text/javascript;charset=utf-8,' + encodeURIComponent(prelude + output);
  let result;
  try {
    const ns = await import(dataUri);
    result = await ns.default.__render();
  } catch (e) {
    console.log(e);
  }
  if (result) {
    const html = Prism.highlight(result, Prism.languages.html, 'html');
    out.innerHTML = html;
  }
}

