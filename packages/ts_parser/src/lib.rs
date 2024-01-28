use oxc_allocator::Allocator;
use oxc_parser::Parser;
use oxc_span::SourceType;
use serde_json;
use wasm_bindgen::prelude::*;

// Instruction:
// create a `test.js`,
// run `cargo run -p oxc_parser --example parser`
// or `cargo watch -x "run -p oxc_parser --example parser"`

// A function that takes a string of TypeScript source code
// and prints its AST in JSON format.
#[wasm_bindgen]
pub fn print_ast(source_text: String) {
    // the source text is always typescript
    const FILE_NAME_OF_TYPE: &str = "template.ts";
    let source_type = SourceType::from_path(FILE_NAME_OF_TYPE).unwrap();

    let allocator = Allocator::default();
    let ret = Parser::new(&allocator, &source_text, source_type).parse();

    if ret.errors.is_empty() {
        println!("{}", serde_json::to_string_pretty(&ret.program).unwrap());
        println!("Parsed Successfully.");
    } else {
        for error in ret.errors {
            let error = error.with_source_code(source_text.clone());
            println!("{error:?}");
        }
    }
}
