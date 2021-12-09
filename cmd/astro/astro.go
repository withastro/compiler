package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	astro "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/printer"
	"github.com/withastro/compiler/internal/transform"
)

func main() {
	source := `
---
import Component from '../components/Component.vue';
export const color = 'red';
export interface Props {
	prop: typeof color
}
export const data = [{ hello: "world" }];

const something = await Astro.fetchContent('../*.md');
---

<html>
  <head>
    <title>Hello {name}</title>
  </head>
  <body>
    <main>
      <Component {...{ "client:load": false }} />
    </main>
	<style define:vars={{ color }}>
		main {
			color: var(--color);
		}
	</style>
  </body>
</html>
`

	doc, err := astro.Parse(strings.NewReader(source))
	if err != nil {
		fmt.Println(err)
		return
	}
	hash := astro.HashFromSource(source)

	transform.ExtractStyles(doc)
	transform.Transform(doc, transform.TransformOptions{
		Scope: hash,
	})

	result := printer.PrintToJS(source, doc, 0, transform.TransformOptions{})

	content, _ := json.Marshal(source)
	sourcemap := `{ "version": 3, "sources": ["file.astro"], "names": [], "mappings": "` + string(result.SourceMapChunk.Buffer) + `", "sourcesContent": [` + string(content) + `] }`
	b64 := base64.StdEncoding.EncodeToString([]byte(sourcemap))
	output := string(result.Output) + string('\n') + `//# sourceMappingURL=data:application/json;base64,` + b64 + string('\n')
	fmt.Print(output)
}

// 	// z := astro.NewTokenizer(strings.NewReader(source))

// 	// for {
// 	// 	if z.Next() == astro.ErrorToken {
// 	// 		// Returning io.EOF indicates success.
// 	// 		return
// 	// 	}
// 	// tok := z.Token()

// 	// if tok.Type == astro.StartTagToken {
// 	// 	for _, attr := range tok.Attr {
// 	// 		switch attr.Type {
// 	// 		case astro.ShorthandAttribute:
// 	// 			fmt.Println("ShorthandAttribute", attr.Key, attr.Val)
// 	// 		case astro.ExpressionAttribute:
// 	// 			if strings.Contains(attr.Val, "<") {
// 	// 				fmt.Println("ExpressionAttribute with Elements", attr.Val)
// 	// 			} else {
// 	// 				fmt.Println("ExpressionAttribute", attr.Key, attr.Val)
// 	// 			}
// 	// 		case astro.QuotedAttribute:
// 	// 			fmt.Println("QuotedAttribute", attr.Key, attr.Val)
// 	// 		case astro.SpreadAttribute:
// 	// 			fmt.Println("SpreadAttribute", attr.Key, attr.Val)
// 	// 		case astro.TemplateLiteralAttribute:
// 	// 			fmt.Println("TemplateLiteralAttribute", attr.Key, attr.Val)
// 	// 		}
// 	// 	}
// 	// }
// 	// }
// }

// func Transform(source string) interface{} {
// 	doc, _ := astro.ParseFragment(strings.NewReader(source), nil)

// 	for _, node := range doc {
// 		fmt.Println(node.Data)
// 	}
// 	// hash := hashFromSource(source)

// 	// transform.Transform(doc, transform.TransformOptions{
// 	// 	Scope: hash,
// 	// })

// 	// w := new(strings.Builder)
// 	// astro.Render(w, doc)
// 	// js := w.String()

// 	// return js
// 	return nil
// }
