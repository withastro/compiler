import { type TransformResult, transform } from '@astrojs/compiler';
import { test } from 'uvu';
import * as assert from 'uvu/assert';

const FIXTURE = `---
// Component Imports
import MainHead from '../components/MainHead.astro';
import Nav from '../components/Nav.astro';
import Footer from '../components/Footer.astro';
import PortfolioPreview from '../components/PortfolioPreview.astro';

// Data Fetching: List all Markdown posts in the repo.
const projects = await Astro.glob('./project/**/*.md');
const featuredProject = projects[0];

// Full Astro Component Syntax:
// https://docs.astro.build/core-concepts/astro-components/
---

<html lang="en">
	<head>
		<MainHead
			title="Jeanine White: Personal Site"
			description="Jeanine White: Developer, Speaker, and Writer..."
		/>

	</head>
	<body>
	    <Nav />
		<small>< header ></small>
	    <Footer />
	</body>
</html>
`;

let result: TransformResult;
test.before(async () => {
	result = await transform(FIXTURE);
});

test('< and > as raw text', () => {
	assert.ok(result.code, 'Expected to compile');
	assert.equal(result.diagnostics.length, 0, 'Expected no diagnostics');
	assert.match(result.code, '< header >', 'Expected output to contain < header >');
});

test.run();
