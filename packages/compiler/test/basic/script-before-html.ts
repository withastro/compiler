import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `
<script>
  // anything ...
</script>

<!DOCTYPE html>
<html lang="de">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width" />
    <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
    <meta name="generator" content={Astro.generator} />
    <title>Astro strips html lang tag</title>
  </head>
  <body>
    <main>
      <slot />
    </main>
  </body>
</html>

<style lang="scss" is:global>
html {
  scroll-behavior: smooth;
}
</style>
`;

let result: unknown;
test.before(async () => {
	result = await transform(FIXTURE);
});

test('includes html element', () => {
	assert.ok(
		result.code.includes('<html lang="de">'),
		'Expected compile result to include html element!'
	);
});

test.run();
