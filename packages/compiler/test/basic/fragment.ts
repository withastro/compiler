import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `
---
import ThemeToggleButton from './ThemeToggleButton.tsx';
---

<title>Uhhh</title>

<body><div>Hello!</div></body>
`;

let result: unknown;
test.before(async () => {
  result = await transform(FIXTURE);
});

test('can compile fragment', () => {
  assert.not.match(result.code, '<head>', 'Expected output not to contain <head>');
  assert.match(result.code, '<body><div>Hello!</div></body>', 'Expected output to contain <body><div>Hello!</div></body>');
});

test.run();
