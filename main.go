package main

import (
	"fmt"
	"log"
	"strings"

	esbuild "github.com/evanw/esbuild/pkg/api"
	tycho "github.com/snowpackjs/tycho/internal"
)

func main() {
	s := `---
// Component Imports
import Counter from '../components/Counter.jsx'
import Block from '../components/Block.jsx'

const result = await fetch('https://google.com/').then(res => res.text());
const Test = 'div';

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
      <Test>Hello world!</Test>
      <Counter client:visible>
        <h1>Hello React!</h1>
      </Counter>
    </main>
  </body>
</html>
`

	doc, err := tycho.Parse(strings.NewReader(s))

	if err != nil {
		log.Fatal(err)
	}

	w := new(strings.Builder)
	tycho.Render(w, doc)
	js := w.String()

	res := esbuild.Transform(js, esbuild.TransformOptions{
		Loader: esbuild.LoaderJS,
	})

	if len(res.Errors) != 0 {
		fmt.Println(js)
		fmt.Println(res.Errors[0].Text)
	}
	fmt.Println(string(res.Code))

}
