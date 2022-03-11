package printer

import (
	"fmt"
	"regexp"
	"strings"

	. "github.com/withastro/compiler/internal"
	"github.com/withastro/compiler/internal/loc"
	"github.com/withastro/compiler/internal/sourcemap"
	"github.com/withastro/compiler/internal/t"
	"github.com/withastro/compiler/internal/transform"
)

type ASTPosition struct {
	Start ASTPoint `json:"start,omitempty"`
	End   ASTPoint `json:"end,omitempty"`
}

type ASTPoint struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
	Offset int `json:"offset,omitempty"`
}

type ASTNode struct {
	Type       string      `json:"type"`
	Name       string      `json:"name"`
	Value      string      `json:"value,omitempty"`
	Attributes []ASTNode   `json:"attributes,omitempty"`
	Directives []ASTNode   `json:"directives,omitempty"`
	Children   []ASTNode   `json:"children,omitempty"`
	Position   ASTPosition `json:"position,omitempty"`

	// Attributes only
	Kind string `json:"kind,omitempty"`
}

func escapeForJSON(value string) string {
	newlines := regexp.MustCompile(`\n`)
	value = newlines.ReplaceAllString(value, `\n`)
	doublequotes := regexp.MustCompile(`"`)
	value = doublequotes.ReplaceAllString(value, `\"`)
	amp := regexp.MustCompile(`&`)
	value = amp.ReplaceAllString(value, `\&`)
	r := regexp.MustCompile(`\r`)
	value = r.ReplaceAllString(value, `\r`)
	t := regexp.MustCompile(`\t`)
	value = t.ReplaceAllString(value, `\t`)
	f := regexp.MustCompile(`\f`)
	value = f.ReplaceAllString(value, `\f`)
	return value
}

func (n ASTNode) String() string {
	str := fmt.Sprintf(`{"type":"%s"`, n.Type)
	if n.Kind != "" {
		str += fmt.Sprintf(`,"kind":"%s"`, n.Kind)
	}
	if n.Name != "" {
		str += fmt.Sprintf(`,"name":"%s"`, escapeForJSON(n.Name))
	} else if n.Type == "fragment" {
		str += `,"name":""`
	}
	if n.Value != "" || n.Type == "attribute" {
		str += fmt.Sprintf(`,"value":"%s"`, escapeForJSON(n.Value))
	}
	if len(n.Attributes) > 0 {
		str += `,"attributes":[`
		for i, attr := range n.Attributes {
			str += attr.String()
			if i < len(n.Attributes)-1 {
				str += ","
			}
		}
		str += `]`
	}
	if len(n.Attributes) == 0 {
		if n.Type == "element" || n.Type == "component" || n.Type == "custom-element" {
			str += `,"attributes":[]`
		}
	}
	if len(n.Directives) > 0 {
		str += `,"directives":[`
		for i, attr := range n.Directives {
			str += attr.String()
			if i < len(n.Directives)-1 {
				str += ","
			}
		}
		str += `]`
	}
	if len(n.Children) > 0 {
		str += `,"children":[`
		for i, node := range n.Children {
			str += node.String()
			if i < len(n.Children)-1 {
				str += ","
			}
		}
		str += `]`
	}
	if n.Position.Start.Line != 0 {
		str += `,"position":{`
		str += fmt.Sprintf(`"start":{"line":%d,"column":%d,"offset":%d}`, n.Position.Start.Line, n.Position.Start.Column, n.Position.Start.Offset)
		if n.Position.End.Line != 0 {
			str += fmt.Sprintf(`,"end":{"line":%d,"column":%d,"offset":%d}`, n.Position.End.Line, n.Position.End.Column, n.Position.End.Offset)
		}
		str += "}"
	}
	str += "}"
	return str
}

func PrintToJSON(sourcetext string, n *Node, opts t.ParseOptions) PrintResult {
	p := &printer{
		builder: sourcemap.MakeChunkBuilder(nil, sourcemap.GenerateLineOffsetTables(sourcetext, len(strings.Split(sourcetext, "\n")))),
	}
	root := ASTNode{}
	renderNode(p, &root, n, opts)
	doc := root.Children[0]
	return PrintResult{
		Output: []byte(doc.String()),
	}
}

func locToPoint(p *printer, loc loc.Loc) ASTPoint {
	offset := loc.Start
	info := p.builder.GetLineAndColumnForLocation(loc)
	line := info[0]
	column := info[1]

	return ASTPoint{
		Line:   line,
		Column: column,
		Offset: offset,
	}
}

func positionAt(p *printer, n *Node, opts t.ParseOptions) ASTPosition {
	if !opts.Position {
		return ASTPosition{}
	}

	if len(n.Loc) == 1 {
		s := n.Loc[0]
		start := locToPoint(p, s)

		return ASTPosition{
			Start: start,
		}
	}

	if len(n.Loc) == 2 {
		s := n.Loc[0]
		e := n.Loc[1]
		start := locToPoint(p, s)
		end := locToPoint(p, e)

		return ASTPosition{
			Start: start,
			End:   end,
		}
	}
	return ASTPosition{}
}

func attrPositionAt(p *printer, n *Attribute, opts t.ParseOptions) ASTPosition {
	if !opts.Position {
		return ASTPosition{}
	}

	k := n.KeyLoc
	start := locToPoint(p, k)

	return ASTPosition{
		Start: start,
	}
}

func renderNode(p *printer, parent *ASTNode, n *Node, opts t.ParseOptions) {
	isImplicit := false
	for _, a := range n.Attr {
		if transform.IsImplictNodeMarker(a) {
			isImplicit = true
			break
		}
	}
	hasChildren := n.FirstChild != nil
	if isImplicit {
		if hasChildren {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				renderNode(p, parent, c, opts)
			}
		}
		return
	}
	var node ASTNode

	node.Position = positionAt(p, n, opts)

	if n.Type == ElementNode {
		if n.Expression {
			node.Type = "expression"
		} else {
			node.Name = n.Data
			if n.Component {
				node.Type = "component"
			} else if n.CustomElement {
				node.Type = "custom-element"
			} else if n.Fragment {
				node.Type = "fragment"
			} else {
				node.Type = "element"
			}

			for _, attr := range n.Attr {
				attrNode := ASTNode{
					Type:     "attribute",
					Kind:     attr.Type.String(),
					Position: attrPositionAt(p, &attr, opts),
					Name:     attr.Key,
					Value:    attr.Val,
				}
				if IsKnownDirective(n, &attr) {
					attrNode.Type = "directive"
					node.Directives = append(node.Directives, attrNode)
				} else {
					node.Attributes = append(node.Attributes, attrNode)
				}
			}
		}
	} else {
		node.Type = n.Type.String()
		if n.Type == TextNode || n.Type == CommentNode || n.Type == DoctypeNode {
			node.Value = n.Data
		}
	}
	if n.Type == FrontmatterNode && hasChildren {
		node.Value = n.FirstChild.Data
	} else {
		if !isImplicit && hasChildren {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				renderNode(p, &node, c, opts)
			}
		}
	}

	parent.Children = append(parent.Children, node)
}
