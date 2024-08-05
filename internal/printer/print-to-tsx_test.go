package printer

import (
	"strings"
	"testing"

	astro "github.com/withastro/compiler/internal"
	handler "github.com/withastro/compiler/internal/handler"
	"github.com/withastro/compiler/internal/transform"
)

func BenchmarkPrintToTSX(b *testing.B) {
	source := `---
import MobileMenu from "$components/layout/MobileMenu.astro";
import MobileMenuSide from "$components/layout/MobileMenuSide.astro";
import Header from "$components/layout/Header.astro";
import Socials from "$components/layout/Socials.astro";
import { getBaseSiteURL } from "$utils";
import { Head } from "astro-capo";
import "src/assets/style/prin.css";
import type { MenuItem } from "../data/sidebarMenu";
import Spritesheet from "$components/Spritesheet.astro";
import GoBackUp from "$components/layout/GoBackUp.astro";

interface Props {
	title?: string;
	description?: string | undefined;
	navItems?: MenuItem[];
	preloadCatalogue?: boolean;
}

const { title, description, navItems, preloadCatalogue } = Astro.props;
const canonicalURL = new URL(Astro.url.pathname, Astro.site);
---

<html lang="en">
	<Head>
		<meta charset="UTF-8" />
		<meta http-equiv="X-UA-Compatible" content="IE=edge" />
		<meta name="viewport" content="width=device-width, initial-scale=1.0" />
		<meta name="generator" content={Astro.generator} />
		<title>Erika</title>

		<link
			rel="preload"
			href="/assets/fonts/IBMPlexResult.woff2"
			as="font"
			type="font/woff2"
			crossorigin
		/>

		<link
			rel="preload"
			href="/assets/fonts/InterResult.woff2"
			as="font"
			type="font/woff2"
			crossorigin
		/>

		{preloadCatalogue && <link rel="preconnect" href="/api/catalogue" crossorigin />}

		<link rel="icon" type="image/svg+xml" href="/favicon.svg" />
		<meta name="description" content={description ? description : "My personal website"} />
		<meta property="og:title" content={title ? title : "Erika"} />
		<meta property="og:description" content={description ? description : "My personal website"} />

		<meta property="og:type" content="website" />
		<meta property="og:site_name" content="erika.florist" />

		<meta property="og:image" content={getBaseSiteURL() + "social-card.png"} />
		<meta name="twitter:card" content="summary" />
		<meta name="twitter:image" content={getBaseSiteURL() + "social-card.png"} />

		<link
			rel="alternate"
			type="application/rss+xml"
			title="Blog"
			href={getBaseSiteURL() + "rss/blog"}
		/>

		<link
			rel="alternate"
			type="application/rss+xml"
			title="Catalogue"
			href={getBaseSiteURL() + "rss/catalogue"}
		/>

		<link rel="canonical" href={canonicalURL} />
		<meta property="og:url" content={canonicalURL} />

		<script src="../assets/scripts/main.ts"></script>
	</Head>
	<body class="bg-black-charcoal">
		<script is:inline>
			const theme = localStorage.getItem("theme"),
				isSystemDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
			theme === "dark" || (!theme && isSystemDark)
				? document.documentElement.classList.add("dark")
				: theme === "light"
					? document.documentElement.classList.remove("dark")
					: theme === "system" && isSystemDark && document.documentElement.classList.add("dark");
		</script>
		<div id="app" class="bg-white-sugar-cane">
			<Header />

			<main>
				<slot />
			</main>

			<footer
				class="m-12 flex justify-center bg-black-charcoal px-5 py-6 leading-tight text-white-sugar-cane sm:m-0 sm:mt-12 sm:px-0"
			>
				<section class="flex w-centered-width justify-between">
					<Socials />

					<div class="prose">
						Powered by <a href="https://astro.build/">Astro</a><br />
						<a href="https://github.com/Princesseuh/erika.florist">Source Code</a><br />
						<a href="/changelog/">Changelog</a>
					</div>
				</section>
			</footer>

			<GoBackUp />
			<MobileMenu />
			<MobileMenuSide navMenu={navItems ?? []} />
		</div>
		<Spritesheet />
	</body>
</html>
`
	for i := 0; i < b.N; i++ {
		h := handler.NewHandler(source, "AstroBenchmark")
		var doc *astro.Node
		doc, err := astro.ParseWithOptions(strings.NewReader(source), astro.ParseOptionWithHandler(h), astro.ParseOptionEnableLiteral(true))
		if err != nil {
			h.AppendError(err)
		}

		PrintToTSX(source, doc, TSXOptions{
			IncludeScripts: false,
			IncludeStyles:  false,
		}, transform.TransformOptions{
			Filename: "AstroBenchmark",
		}, h)
	}
}
