import type { OriginalMapping } from '@jridgewell/trace-mapping';
import * as assert from 'uvu/assert';

import { convertToTSX } from '@astrojs/compiler';
import { originalPositionFor, TraceMap } from '@jridgewell/trace-mapping';
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

function getGeneratedPosition(code: string, snippet: string) {
  const generated = { line: 0, column: 0 };
  let line = 0;
  let column = 0;
  for (let i = 0; i < code.length; i++) {
    const char = code[i];
    if (char === '\n') {
      line++;
      column = 0;
    } else {
      column++;
    }
    if (snippet[0] === char && code.slice(i).startsWith(snippet)) {
      generated.line = line;
      generated.column = column;
    }
  }
  return generated;
}

export async function testSourcemap(input: string, snippet: string) {
  const { code: output, map } = await convertToTSX(input, { sourcemap: 'both', sourcefile: 'index.astro' });
  const tracer = new TraceMap(map);

  console.log(output);

  const generated = getGeneratedPosition(output, snippet);
  const value = input.split('\n')[generated.line - 1].slice(generated.column - 1, generated.column + snippet.length - 1);
  if (value !== snippet) {
    throw new Error(`Unable to match "${snippet}", got "${value}"`);
  }
  const originalPosition = originalPositionFor(tracer, { line: generated.line, column: generated.column - 1 });
  return originalPosition;
}
