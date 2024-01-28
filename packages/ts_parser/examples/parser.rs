use project_root;
use std::path::Path;
use ts_parser::print_ast;

fn main() {
    let root =
        project_root::get_project_root().unwrap_or_else(|_| panic!("Error obtaining project root"));
    let root = root.to_str().unwrap();

    let file_name = "hello-world.ts";
    let file_path = format!("{}/examples/test-ts-files/{file_name}", root);
    let file_path = Path::new(&file_path);

    let source_text =
        std::fs::read_to_string(file_path).unwrap_or_else(|_| panic!("{file_name} not found"));

    print_ast(source_text);
}
