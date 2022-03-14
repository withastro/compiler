function guard(type) {
    return (node) => node.type === type;
}
export const is = {
    parent(node) {
        return Array.isArray(node.children);
    },
    literal(node) {
        return typeof node.value === 'string';
    },
    tag(node) {
        return node.type === 'element' || node.type === 'custom-element' || node.type === 'component' || node.type === 'fragment';
    },
    whitespace(node) {
        return node.type === 'text' && node.value.trim().length === 0;
    },
    root: guard('root'),
    element: guard('element'),
    customElement: guard('custom-element'),
    component: guard('component'),
    fragment: guard('fragment'),
    expression: guard('expression'),
    text: guard('text'),
    doctype: guard('doctype'),
    comment: guard('comment'),
    frontmatter: guard('frontmatter'),
};
class Walker {
    constructor(callback) {
        this.callback = callback;
    }
    async visit(node, parent, index) {
        await this.callback(node, parent, index);
        if (is.parent(node)) {
            let promises = [];
            for (let i = 0; i < node.children.length; i++) {
                const child = node.children[i];
                promises.push(this.callback(child, node, i));
            }
            await Promise.all(promises);
        }
    }
}
export function walk(node, callback) {
    const walker = new Walker(callback);
    walker.visit(node);
}
