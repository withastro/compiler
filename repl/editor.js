const defaultValue = `---
const text = "Hello world!";
---

<html>
  <head>
    <title>Astro</title>
  </head>
  <body>
    <h1>{text}</h1>
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
    if (globalThis.BuildDocument) {
        await renderResult(value);
    }
});

setTimeout(() => {
  renderResult(defaultValue);
}, 250)

const resultRendered = document.querySelector('#output');
const resultHtml = document.querySelector('#html');
const resultJs = document.querySelector('#js');
const resultTitle = document.querySelector('.meta .title');

async function renderResult(source) {
  console.clear();
  source = source.trim();
  const start = performance.now();
  let output;
  try {
    output = await globalThis.BuildDocument(source);
  } catch (e) {
    // console.error(e);
  }
  if (!output) {
    return;
  }
  const endCompile = performance.now();
  console.log(`Compiled in ${Math.floor(endCompile - start)}ms`);
  const js = Prism.highlight(output.trim(), Prism.languages.javascript, 'javascript');
  resultJs.innerHTML = js;
  const dataUri = 'data:text/javascript;charset=utf-8,' + encodeURIComponent(output);
  let result;
  try {
    const ns = await import(dataUri);
    result = await ns.default.__render();
    if (result) {
      resultRendered.srcdoc = result;
      resultRendered.addEventListener('load', () => {
        resultTitle.textContent = resultRendered.contentWindow.document.title;
      })
      const html = Prism.highlight(result, Prism.languages.html, 'html');
      resultHtml.innerHTML = html;
    }
  } catch (e) {
    console.log(e);
  }
}

