//! Astro codegen snapshot tests.
//!
//! Uses `insta::glob!` to discover `.astro` fixture files and compare
//! codegen output against co-located `.snap` snapshot files.

use std::fs;

use astro_codegen::{TransformOptions, transform};
use oxc_allocator::Allocator;
use oxc_parser::Parser;
use oxc_span::SourceType;

fn compile_astro(source: &str) -> String {
    let allocator = Allocator::default();
    let source_type = SourceType::astro();

    let ret = Parser::new(&allocator, source, source_type).parse_astro();

    if !ret.errors.is_empty() {
        let errors: Vec<String> = ret.errors.iter().map(|e| format!("{e}")).collect();
        return format!("Parse errors:\n{}", errors.join("\n"));
    }

    let options = TransformOptions::new()
        .with_internal_url("http://localhost:3000/")
        .with_astro_global_args("\"https://astro.build\"");

    transform(&allocator, source, options, &ret.root).code
}

#[test]
fn snapshots() {
    insta::glob!("fixtures/*.astro", |path| {
        let source_text = fs::read_to_string(path).unwrap();
        let output = compile_astro(&source_text);
        let name = path.file_stem().unwrap().to_str().unwrap();

        insta::with_settings!({
            snapshot_path => path.parent().unwrap(),
            prepend_module_to_snapshot => false,
            snapshot_suffix => "",
            omit_expression => true,
        }, {
            insta::assert_snapshot!(name, output);
        });
    });
}
