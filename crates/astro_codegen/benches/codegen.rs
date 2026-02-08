use astro_codegen::{AstroCodegen, TransformOptions};
use oxc_allocator::Allocator;
use oxc_parser::{ParseOptions, Parser};
use oxc_span::SourceType;

fn default_options() -> TransformOptions {
    TransformOptions {
        filename: Some("Component.astro".to_string()),
        internal_url: Some("http://localhost:3000/".to_string()),
        astro_global_args: Some("\"https://astro.build\"".to_string()),
        ..Default::default()
    }
}

/// Parse once upfront, then benchmark the full codegen pass (scanner + printer).
fn bench_codegen(bencher: divan::Bencher<'_, '_>, source: &str) {
    let allocator = Allocator::default();
    let source_type = SourceType::astro();
    let ret = Parser::new(&allocator, source, source_type)
        .with_options(ParseOptions::default())
        .parse_astro();

    bencher.bench_local(|| {
        let codegen = AstroCodegen::new(&allocator, source, default_options());
        codegen.build(&ret.root)
    });
}

#[divan::bench]
fn favicon(bencher: divan::Bencher<'_, '_>) {
    bench_codegen(bencher, include_str!("fixtures/Favicon.astro"));
}

#[divan::bench]
fn pill_link(bencher: divan::Bencher<'_, '_>) {
    bench_codegen(bencher, include_str!("fixtures/PillLink.astro"));
}

#[divan::bench]
fn seo(bencher: divan::Bencher<'_, '_>) {
    bench_codegen(bencher, include_str!("fixtures/SEO.astro"));
}

#[divan::bench]
fn social_links(bencher: divan::Bencher<'_, '_>) {
    bench_codegen(bencher, include_str!("fixtures/SocialLinks.astro"));
}

#[divan::bench]
fn header_drop_down(bencher: divan::Bencher<'_, '_>) {
    bench_codegen(bencher, include_str!("fixtures/HeaderDropDown.astro"));
}

fn main() {
    divan::main();
}
