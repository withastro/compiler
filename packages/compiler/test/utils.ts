import { convertToTSX, transform } from '@astrojs/compiler';
import { TraceMap, generatedPositionFor, originalPositionFor } from '@jridgewell/trace-mapping';
import sass from 'sass';

export async function preprocessStyle(value, attrs): Promise<any> {
  if (!attrs.lang) {
    return null;
  }
  if (attrs.lang === 'scss') {
    return transformSass(value);
  }
  return null;
}

export function transformSass(value: string) {
  return new Promise((resolve, reject) => {
    sass.render({ data: value }, (err, result) => {
      if (err) {
        reject(err);
        return;
      }
      resolve({ code: result.css.toString('utf8'), map: result.map });
      return;
    });
  });
}

export function getPositionFor(input: string, snippet: string) {
  let index = 0;
  let line = 0;
  let column = 0;
  for (const c of input) {
    if (c === snippet[0] && input.slice(index).startsWith(snippet)) {
      return { line: line + 1, column };
    }
    if (c === '\n') {
      line++;
      column = 0;
    }
    column++;
    index++;
  }
  return null;
}

export async function testTSXSourcemap(input: string, snippet: string) {
  const snippetLoc = getPositionFor(input, snippet);
  if (!snippetLoc) throw new Error(`Unable to find "${snippet}"`);

  const { code, map } = await convertToTSX(input, { sourcemap: 'both', filename: 'index.astro' });
  const tracer = new TraceMap(map);

  const generated = generatedPositionFor(tracer, { source: 'index.astro', line: snippetLoc.line, column: snippetLoc.column });
  if (!generated || generated.line === null) {
    // eslint-disable-next-line no-console
    console.log(code);
    throw new Error(`"${snippet}" position incorrectly mapped in generated output.`);
  }
  const originalPosition = originalPositionFor(tracer, { line: generated.line, column: generated.column });

  return originalPosition;
}

export async function testJSSourcemap(input: string, snippet: string) {
  const snippetLoc = getPositionFor(input, snippet);
  if (!snippetLoc) throw new Error(`Unable to find "${snippet}"`);

  const { code, map } = await transform(input, { sourcemap: 'both', filename: 'index.astro', resolvePath: (i: string) => i });
  const tracer = new TraceMap(map);

  const generated = generatedPositionFor(tracer, { source: 'index.astro', line: snippetLoc.line, column: snippetLoc.column });
  if (!generated || generated.line === null) {
    // eslint-disable-next-line no-console
    console.log(code);
    throw new Error(`"${snippet}" position incorrectly mapped in generated output.`);
  }
  const originalPosition = originalPositionFor(tracer, { line: generated.line, column: generated.column });

  return originalPosition;
}
export const TSXPrefix = '/** @jsxImportSource astro */\n\n';
