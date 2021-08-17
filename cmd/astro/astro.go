package main

// func main() {
// 	source := `---
// 	Hello world!
// 	---
// 	<html client:only { self } { ...spread } {...{ hello: "world" }} data-expr={yo} data-obj={JSON.stringify({ hello: "world" })} client:data-quote="{nice}" data-tagged=` + "`" + `tagged ${literal}` + "`" + `><head></head></html>`

// 	doc, _ := tycho.Parse(strings.NewReader(source))
// 	w := new(strings.Builder)
// 	tycho.Render(w, doc)
// 	fmt.Println(w.String())
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
