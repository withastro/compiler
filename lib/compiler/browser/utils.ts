import {
  Node,
  ParentNode,
  RootNode,
  ElementNode,
  CustomElementNode,
  ComponentNode,
  LiteralNode,
  ExpressionNode,
  TextNode,
  CommentNode,
  DoctypeNode,
  FrontmatterNode,
} from '../shared/ast';

export interface Visitor {
  (node: Node, parent?: ParentNode, index?: number): void | Promise<void>;
}

function guard<Type extends Node>(type: string) {
  return (node: Node): node is Type => node.type === type;
}

export const is = {
  parent(node: Node): node is ParentNode {
    return Array.isArray((node as any).children);
  },
  literal(node: Node): node is LiteralNode {
    return typeof (node as any).value === 'string';
  },
  tag(node: Node): node is ElementNode | CustomElementNode | ComponentNode {
    return node.type === 'element' || node.type === 'custom-element' || node.type === 'component';
  },
  whitespace(node: Node): node is TextNode {
    return node.type === 'text' && node.value.trim().length === 0;
  },
  root: guard<RootNode>('root'),
  element: guard<ElementNode>('element'),
  customElement: guard<CustomElementNode>('custom-element'),
  component: guard<ComponentNode>('component'),
  expression: guard<ExpressionNode>('expression'),
  text: guard<TextNode>('text'),
  doctype: guard<DoctypeNode>('doctype'),
  comment: guard<CommentNode>('comment'),
  frontmatter: guard<FrontmatterNode>('frontmatter'),
};

class Walker {
  constructor(private callback: Visitor) {}
  async visit(node: Node, parent?: ParentNode, index?: number): Promise<void> {
    await this.callback(node, parent, index);
    if (is.parent(node)) {
      let promises = [];
      for (let i = 0; i < node.children.length; i++) {
        const child = node.children[i];
        promises.push(this.callback(child, node as ParentNode, i));
      }
      await Promise.all(promises);
    }
  }
}

export function walk(node: ParentNode, callback: Visitor): void {
  const walker = new Walker(callback);
  walker.visit(node);
}
