import { convertToTSX } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { TSXPrefix } from '../utils';

// https://mathiasbynens.be/notes/javascript-identifiers
const value = `
// Let's goooooo 🚀🚀🚀

// How convenient!
var π = Math.PI;

// Sometimes, you just have to use the Bad Parts of JavaScript:
var ಠ_ಠ = eval;

// Code, Y U NO WORK?!
var ლ_ಠ益ಠ_ლ = 42;

// How about a JavaScript library for functional programming?
var λ = function() {};

// Obfuscate boring variable names for great justice
var \u006C\u006F\u006C\u0077\u0061\u0074 = 'heh';

// …or just make up random ones
var Ꙭൽↈⴱ = 'huh';

// Did you know about the [.] syntax?
var ᱹ = 1;
console.assert([1, 2, 3][ᱹ] === 2);

// While perfectly valid, this doesn’t work in most browsers:
var foo\u200Cbar = 42;

// This is *not* a bitwise left shift (\`<<\`):
var 〱〱 = 2;
// This is, though:
〱〱 << 〱〱; // 8

// Give yourself a discount:
var price_9̶9̶_89 = 'cheap';

// Fun with Roman numerals
var Ⅳ = 4;
var Ⅴ = 5;
Ⅳ + Ⅴ; // 9

// Cthulhu was here
var Hͫ̆̒̐ͣ̊̄ͯ͗͏̵̗̻̰̠̬͝ͅE̴̷̬͎̱̘͇͍̾ͦ͊͒͊̓̓̐_̫̠̱̩̭̤͈̑̎̋ͮͩ̒͑̾͋͘Ç̳͕̯̭̱̲̣̠̜͋̍O̴̦̗̯̹̼ͭ̐ͨ̊̈͘͠M̶̝̠̭̭̤̻͓͑̓̊ͣͤ̎͟͠E̢̞̮̹͍̞̳̣ͣͪ͐̈T̡̯̳̭̜̠͕͌̈́̽̿ͤ̿̅̑Ḧ̱̱̺̰̳̹̘̰́̏ͪ̂̽͂̀͠ = 'Zalgo';`;

test('non-latin characters', async () => {
  const input = `
---
${value}
---

<div></div>
`;
  const output = `${TSXPrefix}
${value}

<Fragment>
<div></div>

</Fragment>
export default function __AstroComponent_(_props: Record<string, any>): any {}\n`;
  const { code } = await convertToTSX(input, { sourcemap: 'external' });
  assert.snapshot(code, output, `expected code to match snapshot`);
});

test.run();
