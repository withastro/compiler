export const render = async (htmlParts: string[], expressions: TemplateStringsArray) => {
  console.log("render", { htmlParts, expressions });
  return 'Not implemented'
}

export interface CompnentCallback {
  (result: any, props: any, slots: any): ReturnType<typeof render>
}

export const createComponent = async (cb: CompnentCallback) => Object.assign(cb, { isAstroComponent: true });

export const renderComponent = (Component: any, props: any, children: any) => {
  console.log("renderComponent", { Component, props, children });
  return '';
}

export const addAttribute = (value: any, key: string) => {
  return ` ${key}="${value}"`;
}

export const spreadAttributes = (values: Record<any, any>) => {
  let output = '';
  for (const [key, value] of Object.entries(values)) {
    output += addAttribute(value, key);
  }
  return output;
}

export const defineStyleVars = (astroId: string, vars: Record<any, any>) => {
  let output = '\n';
  for (const [key, value] of Object.entries(vars)) {
    output += `--${key}: ${value};\n`;
  }
  return `.${astroId} {${output}}`
}
export const defineScriptVars = (vars: Record<any, any>) => {
  let output = '';
  for (const [key, value] of Object.entries(vars)) {
    output += `let ${key} = ${value};\n`;
  }
  return output;
}
