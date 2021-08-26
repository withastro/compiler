package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	astro "github.com/snowpackjs/astro/internal"
	"github.com/snowpackjs/astro/internal/printer"
)

func main() {
	source := `---
import Component from '../components/Component.vue';
const name = "world";
---

<html>
  <head>
    <title>Hello {name}</title>
  </head>
  <body>
    <main>
      <Component client:load />
    </main>
  </body>
</html>
`

	doc, _ := astro.Parse(strings.NewReader(source))
	result := printer.PrintToJS(source, doc)

	content, _ := json.Marshal(source)
	sourcemap := `{ "version": 3, "sources": ["file.astro"], "names": [], "mappings": "` + string(result.SourceMapChunk.Buffer) + `", "sourcesContent": [` + string(content) + `] }`
	b64 := base64.StdEncoding.EncodeToString([]byte(sourcemap))
	output := string(result.Output) + string('\n') + `//# sourceMappingURL=data:application/json;base64,` + b64 + string('\n')
	fmt.Print(output)
}

// 	// z := tycho.NewTokenizer(strings.NewReader(source))

// 	// for {
// 	// 	if z.Next() == tycho.ErrorToken {
// 	// 		// Returning io.EOF indicates success.
// 	// 		return
// 	// 	}
// 	// tok := z.Token()

// 	// if tok.Type == tycho.StartTagToken {
// 	// 	for _, attr := range tok.Attr {
// 	// 		switch attr.Type {
// 	// 		case tycho.ShorthandAttribute:
// 	// 			fmt.Println("ShorthandAttribute", attr.Key, attr.Val)
// 	// 		case tycho.ExpressionAttribute:
// 	// 			if strings.Contains(attr.Val, "<") {
// 	// 				fmt.Println("ExpressionAttribute with Elements", attr.Val)
// 	// 			} else {
// 	// 				fmt.Println("ExpressionAttribute", attr.Key, attr.Val)
// 	// 			}
// 	// 		case tycho.QuotedAttribute:
// 	// 			fmt.Println("QuotedAttribute", attr.Key, attr.Val)
// 	// 		case tycho.SpreadAttribute:
// 	// 			fmt.Println("SpreadAttribute", attr.Key, attr.Val)
// 	// 		case tycho.TemplateLiteralAttribute:
// 	// 			fmt.Println("TemplateLiteralAttribute", attr.Key, attr.Val)
// 	// 		}
// 	// 	}
// 	// }
// 	// }
// }

// func Transform(source string) interface{} {
// 	doc, _ := tycho.ParseFragment(strings.NewReader(source), nil)

// 	for _, node := range doc {
// 		fmt.Println(node.Data)
// 	}
// 	// hash := hashFromSource(source)

// 	// transform.Transform(doc, transform.TransformOptions{
// 	// 	Scope: hash,
// 	// })

// 	// w := new(strings.Builder)
// 	// tycho.Render(w, doc)
// 	// js := w.String()

// 	// return js
// 	return nil
// }
