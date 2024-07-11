import { transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `
---
import Show from './Show.astro';

export interface Props<T> {
	each: Iterable<T>;
}

const { each } = Astro.props;
---

{
	(async function* () {
		for await (const value of each) {
			let html = await Astro.slots.render('default', [value]);
			yield <Fragment set:html={html} />;
			yield '\n';
		}
	})()
}

<Show when={!each.length}>
	<slot name="fallback" />
</Show>
`;

let result: unknown;
test.before(async () => {
  result = await transform(FIXTURE);
});

test('expression followed by node', () => {
  assert.match(
    result.code,
    `yield '
';
		}`,
    'Expected output to properly handle expression!',
  );
});

test.run();
