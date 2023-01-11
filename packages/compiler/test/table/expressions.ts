import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';
import { parse } from 'acorn';

test('allows expressions in table', async () => {
  const input = `
---
---

<html lang="en">
	<head>
		<meta charset="utf-8" />
		<link rel="icon" type="image/svg+xml" href="/favicon.svg" />
		<meta name="viewport" content="width=device-width" />
		<meta name="generator" content={Astro.generator} />
		<title>Astro</title>
	</head>
	<body>
    <table>
      <tbody>
        {[1, 2, 3].map((num) => (
          <tr>{num}</tr>
         ))}
      </tbody>
    </table>
	</body>
</html>
`;

  let error = 0;
  try {
    const { code } = await transform(input, { filename: 'index.astro', sourcemap: 'inline' });
    parse(code, { ecmaVersion: 'latest', sourceType: 'module' });
    assert.match(code, '<tr>${num}</tr>');
  } catch (e) {
    error = 1;
  }
  assert.equal(error, 0, `compiler should generate valid code`);
});

test('allows many expressions in table', async () => {
  const input = `
---
---

<html lang="en">
	<head>
		<meta charset="utf-8" />
		<link rel="icon" type="image/svg+xml" href="/favicon.svg" />
		<meta name="viewport" content="width=device-width" />
		<meta name="generator" content={Astro.generator} />
		<title>Astro</title>
	</head>
	<body>
    <table>
      <tbody>
        {[1, 2, 3].map((num) => (
          <tr>{num}</tr>
         ))}
         {[1, 2, 3].map((num) => (
          <tr>{num}</tr>
         ))}
         {[1, 2, 3].map((num) => (
          <tr>{num}</tr>
         ))}
         {[1, 2, 3].map((num) => (
          <tr>{num}</tr>
         ))}
         {[1, 2, 3].map((num) => (
          <tr>{num}</tr>
         ))}
         {[1, 2, 3].map((num) => (
          <tr>{num}</tr>
         ))}
      </tbody>
    </table>
	</body>
</html>
`;

  let error = 0;
  try {
    const { code } = await transform(input, { filename: 'index.astro', sourcemap: 'inline' });
    parse(code, { ecmaVersion: 'latest', sourceType: 'module' });
    assert.match(code, '<tr>${num}</tr>');
  } catch (e) {
    error = 1;
  }
  assert.equal(error, 0, `compiler should generate valid code`);
});
