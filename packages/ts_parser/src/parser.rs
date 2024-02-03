use oxc_allocator::Allocator;
use oxc_parser::Parser;
use oxc_span::SourceType;
use serde_json;

// A function that takes a string of TypeScript source code
// and prints its AST in JSON format.
pub fn print_ast(source_text: &String) -> String {
    // the source text is always typescript in Astro
    const FILE_NAME_OF_TYPE: &str = "astro.ts";
    let source_type = SourceType::from_path(FILE_NAME_OF_TYPE).unwrap();

    let allocator = Allocator::default();
    let ret = Parser::new(&allocator, source_text, source_type).parse();

    if ret.errors.is_empty() {
        let json_string = format!("{}", serde_json::to_string_pretty(&ret.program).unwrap());
        // console::log_1(&json_string.clone().into());
        return json_string.into();
    } else {
        // console::log_1(&"A TypeScript error occured in your Astro component".into());
        // let's not handle errors for now
        return "{\"hey\": \"there\"}".to_string().into();
        // for error in ret.errors {
        //     let error = error.with_source_code(source_text.clone());
        //     let error = format!("{error:?}");
        //     // console::log_1(&error.into());
        //     return error;
        // }
    }
}
