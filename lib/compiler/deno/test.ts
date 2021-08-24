import { transform, compile } from "https://deno.land/x/astro_compiler@v0.1.0-canary.24/mod.ts";

const template = await transform(`---
import * as o from 'https://deno.land/x/cowsay/mod.ts'
const name = "world";

let m = o.say({
    text: 'hello everyone!!'
})
---

<html>
  <head>
    <title>Hello</title>
  </head>
  <body>
	<main>
		<h1>Hello {name}</h1>
        <pre>{m}</pre>
    </main>
  </body>
</html>`)

const result = await compile(template);

console.log(result);
