# [Experimental] Astro Compiler

### **Note** This is a highly experimental compiler which doesn't really work yet! Consider this a proof-of-concept that is NOT indicative of the future direction of Astro.

## `.astro` => `.js`

Astro's current compiler is stateful and coupled to Snowpack. There's a great opportunity here to improve Astro's separation of concerns and make `.astro` a more general format that can be plugged into different build tools (which may be a non-goal, TBD).

This **experimental** compiler for `.astro` files generates a `.js` module that can be run at server-time. Currently, it is kinda working!

### Todo
- [ ] Make the generated code run in Node 😅
- [ ] Support fragments (non-Page components)
- [ ] Extract styles
- [ ] Compile TS to JS
- [ ] Handle markup in embedded expressions
- [ ] import Astro's `h` function
- [ ] Figure out what to do with top-level `await`, `exports`
- [ ] Link generated template to `__render` function with slots
- [ ] Attach JS variables to `$$data` namespace


You can demo it by running

```shell
go run .
```

## Input
```astro
---
// Component Imports
import Counter from '../components/Counter.jsx'

// Full Astro Component Syntax:
// https://docs.astro.build/core-concepts/astro-components/
---
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta
      name="viewport"
      content="width=device-width, initial-scale=1, viewport-fit=cover"
    />
    <style>
      :global(:root) {
        font-family: system-ui;
        padding: 2em 0;
      }
      :global(.counter) {
        display: grid;
        grid-template-columns: repeat(3, minmax(0, 1fr));
        place-items: center;
        font-size: 2em;
        margin-top: 2em;
      }
      :global(.children) {
        display: grid;
        place-items: center;
        margin-bottom: 2em;
      }
    </style>
  </head>
  <body>
    <main>
      <Counter client:visible>
        <h1>Hello React!</h1>
      </Counter>
    </main>
  </body>
</html>
```

## Output 
```js

// Component Imports
import Counter from '../components/Counter.jsx'

// Full Astro Component Syntax:
// https://docs.astro.build/core-concepts/astro-components/

const __renderTemplate = ($$data) => (h("html",{"lang":"en"},h("head",null,`
    `,h("meta",{"charset":"utf-8"}),`
    `,h("meta",{"name":"viewport","content":"width=device-width, initial-scale=1, viewport-fit=cover"}),`
    `,h("style",null,`
      :global(:root) {
        font-family: system-ui;
        padding: 2em 0;
      }
      :global(.counter) {
        display: grid;
        grid-template-columns: repeat(3, minmax(0, 1fr));
        place-items: center;
        font-size: 2em;
        margin-top: 2em;
      }
      :global(.children) {
        display: grid;
        place-items: center;
        margin-bottom: 2em;
      }
    `),`
  `),`
  `,h("body",null,`
    `,h("main",null,`
      `,h(__astro_component,null,h(Counter,{"client:visible":""},`
        `,h("h1",null,`Hello React!`),`
      `),`
    `),`
  

`))
```
