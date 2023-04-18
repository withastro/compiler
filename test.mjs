import { convertToTSX } from './packages/compiler/node/index.js';
import fs from 'node:fs';

async function run() {
    const file = fs.readFileSync('/Users/nmoo/Desktop/render.astro', 'utf8').replace(/\n/g, '\r\n');
    const output = await convertToTSX(file, { sourcemap: 'inline' })
    fs.writeFileSync('/Users/nmoo/Desktop/render.astro.js', output.code, 'utf8');
}

run();
