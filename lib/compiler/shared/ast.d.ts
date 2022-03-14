export declare type ParentNode = RootNode | ElementNode | ComponentNode | CustomElementNode | FragmentNode | ExpressionNode;
export declare type Node = RootNode | ElementNode | ComponentNode | CustomElementNode | FragmentNode | ExpressionNode | TextNode | FrontmatterNode | DoctypeNode | CommentNode;
export interface Position {
    start: Point;
    end?: Point;
}
export interface Point {
    /** 1-based line number */
    line: number;
    /** 1-based column number, per-line */
    column: number;
    /** 0-based byte offset */
    offset: number;
}
export interface BaseNode {
    type: string;
    position?: Position;
}
export interface ParentLikeNode extends BaseNode {
    type: 'element' | 'component' | 'custom-element' | 'fragment' | 'expression' | 'root';
    children: Node[];
}
export interface LiteralNode extends BaseNode {
    type: 'text' | 'doctype' | 'comment' | 'frontmatter';
    value: string;
}
export interface RootNode extends ParentLikeNode {
    type: 'root';
}
export interface AttributeNode extends BaseNode {
    type: 'attribute';
    kind: 'quoted' | 'empty' | 'expression' | 'spread' | 'shorthand' | 'template-literal';
    name: string;
    value: string;
}
export interface DirectiveNode extends Omit<AttributeNode, 'type'> {
    type: 'directive';
}
export interface TextNode extends LiteralNode {
    type: 'text';
}
export interface ElementNode extends ParentLikeNode {
    type: 'element';
    name: string;
    attributes: AttributeNode[];
    directives: DirectiveNode[];
}
export interface FragmentNode extends ParentLikeNode {
    type: 'fragment';
    name: string;
    attributes: AttributeNode[];
    directives: DirectiveNode[];
}
export interface ComponentNode extends ParentLikeNode {
    type: 'component';
    name: string;
    attributes: AttributeNode[];
    directives: DirectiveNode[];
}
export interface CustomElementNode extends ParentLikeNode {
    type: 'custom-element';
    name: string;
    attributes: AttributeNode[];
    directives: DirectiveNode[];
}
export interface DoctypeNode extends LiteralNode {
    type: 'doctype';
}
export interface CommentNode extends LiteralNode {
    type: 'comment';
}
export interface FrontmatterNode extends LiteralNode {
    type: 'frontmatter';
}
export interface ExpressionNode extends ParentLikeNode {
    type: 'expression';
}
