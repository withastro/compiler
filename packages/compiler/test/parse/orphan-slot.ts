import { test } from 'uvu';
import * as assert from 'uvu/assert';
import { transform } from '@astrojs/compiler';

const FIXTURE = `
---
import { Code, Markdown } from 'astro/components';

const {isRequired, description, example} = Astro.props;
---

<slot />
{isRequired && <p class="mt-16 badge badge-info">Required</p>}
{description?.trim() && <Markdown content={description} />}
{example && <Code code={example} lang='yaml' />}
`;

let result;
test.before(async () => {
  result = await transform(FIXTURE);
});

test('orphan slot', () => {
  assert.ok(result.code, 'able to parse');
});

test.run();
