import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `---
---

<span class="spoiler">
    <slot />
</span>

<style>
span { color: red; }
</style>
<script>
console.log("hello")
</script>
`;

let result: TransformResult;
test.before(async () => {
	result = await transform(FIXTURE);
});

test('trailing space', () => {
	assert.ok(result.code, 'Expected to compiler');
	assert.match(
		result.code,
		`<span class="spoiler astro-bqati2k5">
    \${$$renderSlot($$result,$$slots["default"])}
</span>


\${$$renderScript($$result,"<stdin>?astro&type=script&index=0&lang.ts")}\``
	);
});

test.run();
