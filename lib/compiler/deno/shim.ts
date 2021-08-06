// @ts-nocheck
const voidTags = new Set(["area", "base", "br", "col", "command", "embed", "hr", "img", "input", "keygen", "link", "meta", "param", "source", "track", "wbr"]);
function* _h(tag, attrs, children) {
  if (tag.toLowerCase() === "!doctype") {
    yield `<${tag} `;
    if (attrs) {
      yield Object.keys(attrs).join(" ");
    }
    yield ">";
    return;
  }
  yield `<${tag}`;
  if (attrs) {
    for (let [key, value] of Object.entries(attrs)) {
      if (value === "")
        yield ` ${key}=""`;
      else if (value == null || value === false)
        yield "";
      else if (value === true)
        yield ` ${key}`;
      else
        yield ` ${key}="${value}"`;
    }
  }
  yield ">";
  if (voidTags.has(tag)) {
    return;
  }
  for (let child of children) {
    if (typeof child === "function") {
      yield child();
    } else if (typeof child === "string") {
      yield child;
    } else if (!child && child !== 0) {
    } else {
      yield child;
    }
  }
  yield `</${tag}>`;
}
async function h(tag, attrs, ...pChildren) {
  const children = await Promise.all(pChildren.flat(Infinity));
  if (typeof tag === "function") {
    return tag(attrs, ...children);
  }
  return Array.from(_h(tag, attrs, children)).join("");
}
function Fragment(_, ...children) {
  console.log(children);
  return children.join("");
}
function __astro_slot_content({name}, ...children) {
  return {$slot: name, children};
}
const __astro_slot = ({name = "default"}, _children, ...fallback) => {
  if (name === "default" && typeof _children === "string") {
    return _children ? _children : fallback;
  }
  if (!_children.$slots) {
    throw new Error(`__astro_slot encountered an unexpected child:
${JSON.stringify(_children)}`);
  }
  const children = _children.$slots[name];
  return children ? children : fallback;
};

// @ts-ignore
globalThis.__astro_slot = __astro_slot;
globalThis.__astro_slot_content = __astro_slot_content;
globalThis.Fragment = Fragment;
globalThis.h = h;
globalThis.Astro = {};
globalThis.__astro_component = async (context, ...children) => {
  const result = await context.Component();
  console.log('__astro_component', context.Component.toString());
  // const { Component } = context;
  // const output = renderToString(html`<${Component}>...${children}</${Component}>`);
  return `<astro-root>${result}</astro-root>`;
}
