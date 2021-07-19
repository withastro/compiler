package main

import (
	"fmt"
	"log"
	"strings"

	astro "github.com/snowpackjs/go-astro/internal"
)

func main() {
	s := `---
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
`

	doc, err := astro.Parse(strings.NewReader(s))

	if err != nil {
		log.Fatal(err)
	}

	w := new(strings.Builder)
	astro.Render(w, doc)

	fmt.Println(w.String())

}
