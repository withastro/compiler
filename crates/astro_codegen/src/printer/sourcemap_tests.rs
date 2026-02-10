// --- Sourcemap tests ---

// Test sources are small; line/column counts never exceed u32.
#![allow(
    clippy::cast_possible_truncation,
    clippy::cast_sign_loss
)]

use crate::{TransformOptions, TransformResult, printer::tests::compile_astro_with_options};

fn compile_astro_with_sourcemap(source: &str) -> TransformResult {
    compile_astro_with_options(
        source,
        TransformOptions::new()
            .with_internal_url("http://localhost:3000/")
            .with_sourcemap(true)
            .with_filename("test.astro"),
    )
}

#[test]
fn test_sourcemap_enabled_produces_nonempty_map() {
    let source = "<h1>Hello</h1>";
    let result = compile_astro_with_sourcemap(source);
    assert!(
        !result.map.is_empty(),
        "sourcemap should be non-empty when enabled"
    );
}

#[test]
fn test_sourcemap_disabled_produces_empty_map() {
    let source = "<h1>Hello</h1>";
    let result = compile_astro_with_options(
        source,
        TransformOptions::new().with_internal_url("http://localhost:3000/"),
    );
    assert!(
        result.map.is_empty(),
        "sourcemap should be empty when disabled"
    );
}

#[test]
fn test_sourcemap_is_valid_json() {
    let source = "<h1>Hello</h1>";
    let result = compile_astro_with_sourcemap(source);
    let parsed: serde_json::Value =
        serde_json::from_str(&result.map).expect("sourcemap should be valid JSON");
    assert_eq!(parsed["version"], 3, "sourcemap version should be 3");
}

#[test]
fn test_sourcemap_has_correct_source_filename() {
    let source = "<h1>Hello</h1>";
    let result = compile_astro_with_sourcemap(source);
    let parsed: serde_json::Value = serde_json::from_str(&result.map).unwrap();
    let sources = parsed["sources"]
        .as_array()
        .expect("sources should be an array");
    assert_eq!(sources.len(), 1);
    assert_eq!(sources[0], "test.astro");
}

#[test]
fn test_sourcemap_has_sources_content() {
    let source = "<h1>Hello</h1>";
    let result = compile_astro_with_sourcemap(source);
    let parsed: serde_json::Value = serde_json::from_str(&result.map).unwrap();
    let contents = parsed["sourcesContent"]
        .as_array()
        .expect("sourcesContent should be an array");
    assert_eq!(contents.len(), 1);
    assert_eq!(contents[0], source);
}

#[test]
fn test_sourcemap_has_mappings() {
    let source = "<h1>Hello</h1>";
    let result = compile_astro_with_sourcemap(source);
    let parsed: serde_json::Value = serde_json::from_str(&result.map).unwrap();
    let mappings = parsed["mappings"]
        .as_str()
        .expect("mappings should be a string");
    assert!(
        !mappings.is_empty(),
        "mappings should be non-empty for template content"
    );
}

#[test]
fn test_sourcemap_with_frontmatter() {
    let source = r#"---
const name = "World";
---
<h1>Hello {name}!</h1>"#;
    let result = compile_astro_with_sourcemap(source);

    assert!(!result.map.is_empty(), "sourcemap should be non-empty");
    let parsed: serde_json::Value = serde_json::from_str(&result.map).unwrap();
    assert_eq!(parsed["version"], 3);
    assert_eq!(parsed["sources"][0], "test.astro");

    // The source content should contain the original .astro source
    assert_eq!(parsed["sourcesContent"][0], source);
}

#[test]
fn test_sourcemap_with_expressions() {
    let source = r"---
const items = [1, 2, 3];
---
<ul>
{items.map(i => <li>{i}</li>)}
</ul>";
    let result = compile_astro_with_sourcemap(source);

    assert!(!result.map.is_empty(), "sourcemap should be non-empty");
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map)
        .expect("should parse as valid sourcemap");

    // Should have at least some tokens mapping back to the source
    assert!(
        sm.get_tokens().next().is_some(),
        "sourcemap should contain at least one mapping token"
    );
}

#[test]
fn test_sourcemap_with_component() {
    let source = r#"---
import MyComponent from './MyComponent.astro';
---
<MyComponent name="test" />"#;
    let result = compile_astro_with_sourcemap(source);

    assert!(!result.map.is_empty(), "sourcemap should be non-empty");
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map)
        .expect("should parse as valid sourcemap");

    assert!(
        sm.get_tokens().next().is_some(),
        "sourcemap should contain mapping tokens for component"
    );
}

#[test]
fn test_sourcemap_points_to_original_lines() {
    let source = r"---
const x = 1;
---
<div>hello</div>";
    let result = compile_astro_with_sourcemap(source);

    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map)
        .expect("should parse as valid sourcemap");

    // Find a token that maps to line 3 (0-indexed) in the original source,
    // which is the `<div>hello</div>` line
    let tokens: Vec<_> = sm.get_tokens().collect();
    let has_template_mapping = tokens.iter().any(|t| t.get_src_line() == 3);
    assert!(
        has_template_mapping,
        "sourcemap should have at least one mapping pointing to the template line (line 3). Tokens: {tokens:?}"
    );
}

#[test]
fn test_sourcemap_frontmatter_statement_mapping() {
    let source = r#"---
const greeting = "hello";
---
<p>{greeting}</p>"#;
    let result = compile_astro_with_sourcemap(source);

    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map)
        .expect("should parse as valid sourcemap");

    let tokens: Vec<_> = sm.get_tokens().collect();
    // The `const greeting = "hello"` is on line 1 (0-indexed) of the original source
    let has_frontmatter_mapping = tokens.iter().any(|t| t.get_src_line() == 1);
    assert!(
        has_frontmatter_mapping,
        "sourcemap should map frontmatter statement back to original line 1. Tokens: {tokens:?}"
    );
}

#[test]
fn test_sourcemap_visualizer_basic() {
    let source = r#"---
const name = "World";
---
<h1>Hello {name}!</h1>"#;
    let result = compile_astro_with_sourcemap(source);

    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map)
        .expect("should parse as valid sourcemap");

    let visualizer = oxc_sourcemap::SourcemapVisualizer::new(&result.code, &sm);
    let text = visualizer.get_text();

    // The visualizer output should be non-empty and reference our source file
    assert!(
        !text.is_empty(),
        "sourcemap visualizer text should not be empty"
    );
    assert!(
        text.contains("test.astro"),
        "visualizer should reference the source file. Got:\n{text}"
    );
}

#[test]
fn test_sourcemap_with_typescript_stripping() {
    let source = r"---
interface Props {
title: string;
}
const props: Props = Astro.props;
---
<h1>{props.title}</h1>";
    let result = compile_astro_with_sourcemap(source);

    // TypeScript should be stripped
    assert!(
        !result.code.contains("interface Props"),
        "interface should be stripped: {}",
        result.code
    );

    // Sourcemap should still be valid after TS stripping
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map)
        .expect("sourcemap should be valid JSON after TS stripping");

    assert!(
        sm.get_tokens().next().is_some(),
        "sourcemap should have tokens after TS stripping"
    );

    // Should still map back to the original .astro source
    assert_eq!(
        sm.get_sources().next().map(std::convert::AsRef::as_ref),
        Some("test.astro")
    );
}

/// Open the sourcemap visualizer in your browser.
///
/// Run with: `cargo test -p astro_codegen -- test_sourcemap_open_visualizer --ignored --nocapture`
///
/// Change `source` to whatever `.astro` content you want to inspect.
#[test]
#[ignore = "opens a browser — run manually"]
#[expect(clippy::print_stderr)]
fn test_sourcemap_open_visualizer() {
    let source = r#"---
console.log(
	'Hello from Astro! This page was rendered on the server, and sent to the client as HTML. You can view the source code to see how it works!',
)();
---

<html lang="en">
	<head>
		<meta charset="utf-8" />
		<link rel="icon" type="image/svg+xml" href="/favicon.svg" />
		<link rel="icon" href="/favicon.ico" />
		<meta name="viewport" content="width=device-width" />
		<meta name="generator" content={Astro.generator} />
		<title>Astro</title>
	</head>
	<body>
		<h1>Astro</h1>
	</body>
</html>"#;
    let result = compile_astro_with_options(
        source,
        TransformOptions::new()
            .with_internal_url("http://localhost:3000/")
            .with_sourcemap(true)
            .with_filename("rust-compiler.astro"),
    );
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map)
        .expect("should parse as valid sourcemap");
    let vis = oxc_sourcemap::SourcemapVisualizer::new(&result.code, &sm);
    let url = vis.get_url();
    eprintln!("Opening sourcemap visualizer...\n{url}");
    let _ = std::process::Command::new("xdg-open").arg(&url).spawn();
    // Give the browser a moment to open before the process exits.
    std::thread::sleep(std::time::Duration::from_secs(2));
}

#[test]
fn test_sourcemap_no_frontmatter() {
    let source = "<div><span>Just HTML</span></div>";
    let result = compile_astro_with_sourcemap(source);

    assert!(!result.map.is_empty(), "sourcemap should be non-empty");
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map)
        .expect("should parse as valid sourcemap");

    let tokens: Vec<_> = sm.get_tokens().collect();
    // All tokens should map to line 0 since the source is a single line
    let has_line_0 = tokens.iter().any(|t| t.get_src_line() == 0);
    assert!(
        has_line_0,
        "sourcemap should have tokens mapping to line 0. Tokens: {tokens:?}"
    );
}

// --- Stress tests: validate actual mapping correctness ---

/// Helper: for each sourcemap token, resolve the original text snippet it
/// points to and the generated text snippet, so we can assert they make
/// sense together.
fn visualize_mappings(code: &str, map_json: &str) -> String {
    let sm = oxc_sourcemap::SourceMap::from_json_string(map_json)
        .expect("should parse as valid sourcemap");
    let vis = oxc_sourcemap::SourcemapVisualizer::new(code, &sm);
    vis.get_text()
}

/// Helper: get lines of original source text by 0-indexed line number.
fn source_line(source: &str, line: u32) -> &str {
    source
        .lines()
        .nth(line as usize)
        .unwrap_or("<out of bounds>")
}

#[test]
fn test_sourcemap_tokens_never_point_past_source() {
    // Regression: make sure no token points to a line/col beyond the
    // original source boundaries.
    let source = r"---
const a = 1;
---
<p>hi</p>";
    let result = compile_astro_with_sourcemap(source);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();

    let src_line_count = source.lines().count() as u32;
    for token in sm.get_tokens() {
        assert!(
            token.get_src_line() < src_line_count,
            "token src_line {} >= source line count {}. Token: {:?}",
            token.get_src_line(),
            src_line_count,
            token
        );
        let line_text = source_line(source, token.get_src_line());
        // Column is UTF-16 code units; for ASCII this equals byte length.
        // Allow equal (pointing to end-of-line) but not greater.
        let line_utf16_len = line_text.encode_utf16().count() as u32;
        assert!(
            token.get_src_col() <= line_utf16_len,
            "token src_col {} > line utf16 len {} on line {}. Line text: {:?}. Token: {:?}",
            token.get_src_col(),
            line_utf16_len,
            token.get_src_line(),
            line_text,
            token
        );
    }
}

#[test]
fn test_sourcemap_tokens_never_point_past_generated() {
    let source = r"---
const x = 42;
---
<div>{x}</div>";
    let result = compile_astro_with_sourcemap(source);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();

    let gen_line_count = result.code.lines().count() as u32;
    for token in sm.get_tokens() {
        assert!(
            token.get_dst_line() < gen_line_count,
            "token dst_line {} >= generated line count {}. Token: {:?}",
            token.get_dst_line(),
            gen_line_count,
            token
        );
    }
}

#[test]
fn test_sourcemap_frontmatter_variable_maps_to_correct_line() {
    // The variable `const name = "World"` is on line 1 (0-indexed) of the
    // astro source. The generated code should map back there, not to line 0
    // (the `---` fence) or line 2 (the closing `---`).
    let source = "---\nconst name = \"World\";\n---\n<h1>{name}</h1>";
    let result = compile_astro_with_sourcemap(source);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();

    // Find a token whose generated text looks like the `name` variable
    // (it should appear in the generated code somewhere around `const name`)
    let gen_lines: Vec<&str> = result.code.lines().collect();
    let const_gen_line = gen_lines
        .iter()
        .position(|l| l.contains("const name"))
        .expect("generated code should contain `const name`");

    let tokens: Vec<_> = sm.get_tokens().collect();
    let matching = tokens
        .iter()
        .filter(|t| t.get_dst_line() == const_gen_line as u32)
        .collect::<Vec<_>>();

    assert!(
        !matching.is_empty(),
        "no sourcemap token on generated line {const_gen_line} which has `const name`"
    );

    // All such tokens should map back to line 1 in the original source
    for t in &matching {
        assert_eq!(
            t.get_src_line(),
            1,
            "token on `const name` line should map to original line 1 (the statement), not {}. \
             Generated line: {:?}. Token: {:?}",
            t.get_src_line(),
            gen_lines[const_gen_line],
            t
        );
    }
}

#[test]
fn test_sourcemap_expression_maps_back_to_template_line() {
    // {items.length} is on line 3 (0-indexed) of the original.
    let source = "---\nconst items = [1,2];\n---\n<p>{items.length}</p>";
    let result = compile_astro_with_sourcemap(source);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();

    let tokens: Vec<_> = sm.get_tokens().collect();
    // At least one token should map to original line 3
    let has_line3 = tokens.iter().any(|t| t.get_src_line() == 3);
    assert!(
        has_line3,
        "should have a mapping back to line 3 (the template line). Tokens: {tokens:?}"
    );
}

#[test]
fn test_sourcemap_multiline_template_each_line_mapped() {
    let source = "---\nconst a = 1;\n---\n<h1>first</h1>\n<h2>second</h2>\n<h3>third</h3>";
    let result = compile_astro_with_sourcemap(source);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let tokens: Vec<_> = sm.get_tokens().collect();

    // Lines 3, 4, 5 in the original each have an element.
    // We should have at least one mapping for each.
    for expected_line in [3u32, 4, 5] {
        let found = tokens.iter().any(|t| t.get_src_line() == expected_line);
        assert!(
            found,
            "expected at least one mapping to original line {expected_line}. Tokens: {tokens:?}"
        );
    }
}

#[test]
fn test_sourcemap_empty_source() {
    let source = "";
    let result = compile_astro_with_sourcemap(source);
    // Should not panic, map may or may not be empty
    if !result.map.is_empty() {
        let _sm = oxc_sourcemap::SourceMap::from_json_string(&result.map)
            .expect("if map is non-empty it should be valid JSON");
    }
}

#[test]
fn test_sourcemap_only_frontmatter_no_template() {
    let source = "---\nconst x = 1;\n---";
    let result = compile_astro_with_sourcemap(source);
    assert!(!result.map.is_empty());
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let tokens: Vec<_> = sm.get_tokens().collect();
    // The frontmatter statement should still be mapped
    let has_line1 = tokens.iter().any(|t| t.get_src_line() == 1);
    assert!(
        has_line1,
        "frontmatter-only file should still map `const x = 1` to line 1. Tokens: {tokens:?}"
    );
}

#[test]
fn test_sourcemap_deeply_nested_expression() {
    let source = r#"---
const data = { items: [{ name: "a" }] };
---
<ul>
{data.items.map(item => <li>{item.name}</li>)}
</ul>"#;
    let result = compile_astro_with_sourcemap(source);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let tokens: Vec<_> = sm.get_tokens().collect();

    // The expression is on line 4; we should have at least one mapping there
    let has_line4 = tokens.iter().any(|t| t.get_src_line() == 4);
    assert!(
        has_line4,
        "nested expression on line 4 should be mapped. Tokens: {tokens:?}"
    );
}

#[test]
fn test_sourcemap_generated_code_has_no_mapping_to_fence_lines() {
    // Lines 0 and 2 are `---` fences; nothing in the generated code should
    // map to those lines because fences are not emitted.
    let source = "---\nconst x = 1;\n---\n<p>{x}</p>";
    let result = compile_astro_with_sourcemap(source);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let tokens: Vec<_> = sm.get_tokens().collect();

    for t in &tokens {
        assert_ne!(
            t.get_src_line(),
            0,
            "no token should map to line 0 (opening ---). Token: {t:?}"
        );
        assert_ne!(
            t.get_src_line(),
            2,
            "no token should map to line 2 (closing ---). Token: {t:?}"
        );
    }
}

#[test]
fn test_sourcemap_visualizer_sanity() {
    // Use the visualizer to do a basic sanity check: every line of the
    // visualizer output that references `test.astro` should have a
    // reasonable format.
    let source = "---\nconst x = 1;\n---\n<div>{x}</div>";
    let result = compile_astro_with_sourcemap(source);
    let text = visualize_mappings(&result.code, &result.map);
    for line in text.lines() {
        if line.starts_with('(') {
            // Format: (src_line:src_col) "original" --> (dst_line:dst_col) "generated"
            assert!(
                line.contains("-->"),
                "visualizer mapping line should contain `-->`: {line}"
            );
        }
    }
}

#[test]
fn test_sourcemap_unicode_content() {
    let source = "---\nconst emoji = \"🎉\";\n---\n<p>{emoji} héllo</p>";
    let result = compile_astro_with_sourcemap(source);
    // Should not panic
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let tokens: Vec<_> = sm.get_tokens().collect();
    assert!(
        !tokens.is_empty(),
        "unicode content should still produce tokens"
    );

    // Verify no token points beyond source boundaries
    let src_line_count = source.lines().count() as u32;
    for t in &tokens {
        assert!(
            t.get_src_line() < src_line_count,
            "unicode: token line {} out of bounds ({}). Token: {t:?}",
            t.get_src_line(),
            src_line_count
        );
    }
}

#[test]
fn test_sourcemap_large_frontmatter() {
    // Lots of statements — make sure they all get individual mappings
    let source = "---\n\
        const a = 1;\n\
        const b = 2;\n\
        const c = 3;\n\
        const d = 4;\n\
        const e = 5;\n\
        ---\n\
        <p>{a}{b}{c}{d}{e}</p>";
    let result = compile_astro_with_sourcemap(source);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let tokens: Vec<_> = sm.get_tokens().collect();

    // Each const is on lines 1..=5; we should map each one
    for expected_line in 1u32..=5 {
        let found = tokens.iter().any(|t| t.get_src_line() == expected_line);
        assert!(
            found,
            "should map `const` on line {expected_line}. Tokens: {tokens:?}"
        );
    }
}

// ---------------------------------------------------------------
// Helper utilities for sourcemap assertion tests
// ---------------------------------------------------------------

/// Collect all composed tokens as a vec of (dst_line, dst_col, src_line, src_col).
fn token_tuples(map_json: &str) -> Vec<(u32, u32, u32, u32)> {
    let sm = oxc_sourcemap::SourceMap::from_json_string(map_json).expect("valid sourcemap JSON");
    sm.get_tokens()
        .map(|t| {
            (
                t.get_dst_line(),
                t.get_dst_col(),
                t.get_src_line(),
                t.get_src_col(),
            )
        })
        .collect()
}

/// Return true if any token maps to the given source line.
fn has_src_line(tokens: &[(u32, u32, u32, u32)], line: u32) -> bool {
    tokens.iter().any(|t| t.2 == line)
}

/// Get the generated-code text at a token's position (up to `len` chars).
fn gen_text_at(code: &str, dst_line: u32, dst_col: u32, len: usize) -> String {
    code.lines()
        .nth(dst_line as usize)
        .map(|l| {
            let start = (dst_col as usize).min(l.len());
            let end = (start + len).min(l.len());
            l[start..end].to_string()
        })
        .unwrap_or_default()
}

/// Get the source text at a token's source position (up to `len` chars).
fn src_text_at(source: &str, src_line: u32, src_col: u32, len: usize) -> String {
    source
        .lines()
        .nth(src_line as usize)
        .map(|l| {
            let start = (src_col as usize).min(l.len());
            let end = (start + len).min(l.len());
            l[start..end].to_string()
        })
        .unwrap_or_default()
}

/// Assert that no token ever points past the source or generated bounds.
fn assert_tokens_in_bounds(source: &str, result: &TransformResult) {
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let src_line_count = source.lines().count() as u32;
    let gen_line_count = result.code.lines().count() as u32;
    for t in sm.get_tokens() {
        assert!(
            t.get_src_line() < src_line_count,
            "src_line {} >= source lines {}. Token gen {}:{}",
            t.get_src_line(),
            src_line_count,
            t.get_dst_line(),
            t.get_dst_col()
        );
        assert!(
            t.get_dst_line() < gen_line_count,
            "dst_line {} >= generated lines {}. Token src {}:{}",
            t.get_dst_line(),
            gen_line_count,
            t.get_src_line(),
            t.get_src_col()
        );
        let src_line_text = source_line(source, t.get_src_line());
        let src_utf16_len = src_line_text.encode_utf16().count() as u32;
        assert!(
            t.get_src_col() <= src_utf16_len,
            "src_col {} > line len {} on src line {}",
            t.get_src_col(),
            src_utf16_len,
            t.get_src_line()
        );
    }
}

// ---------------------------------------------------------------
// Tests for the closing-tag mapping fix
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_closing_tags_have_mappings() {
    // Every closing tag (</h1>, </p>, </ul>, </li>, </div>) should have
    // a dedicated mapping pointing to its source position.
    let source = r"---
const x = 1;
---
<div>
<h1>Title</h1>
<p>Paragraph</p>
<ul>
<li>item</li>
</ul>
</div>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Frontmatter line 1
    assert!(
        has_src_line(&tokens, 1),
        "frontmatter line 1 should be mapped"
    );
    // Template line 3 (<div>)
    assert!(
        has_src_line(&tokens, 3),
        "template line 3 <div> should be mapped"
    );
    // Expression line 4 (ternary with JSX)
    assert!(
        has_src_line(&tokens, 4),
        "ternary expression on line 4 should be mapped"
    );
    // Closing </div> on line 5
    assert!(
        has_src_line(&tokens, 5),
        "closing </div> on line 5 should be mapped"
    );
}

#[test]
fn test_sourcemap_conditional_closing_tags() {
    // Both branches of a ternary produce closing tags that should be mapped.
    let source = r"---
const x = true;
---
{x ? <p>yes</p> : <p>no</p>}";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();

    // Source line 3: `{x ? <p>yes</p> : <p>no</p>}`
    // </p> appears at col 12 and col 24
    let close_tags: Vec<_> = sm
        .get_tokens()
        .filter(|t| {
            let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 4);
            src == "</p>"
        })
        .collect();
    assert!(
        close_tags.len() >= 2,
        "should map both </p> closing tags in ternary, found {}",
        close_tags.len()
    );
}

// ---------------------------------------------------------------
// Complex file: deeply nested HTML elements
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_deeply_nested_html() {
    let source = r"<div>
<section>
<article>
  <header>
    <h2>Title</h2>
  </header>
  <p>Content</p>
</article>
</section>
</div>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let tokens: Vec<_> = sm.get_tokens().collect();

    // Every source line should have at least one mapping (they all contain
    // either an opening or closing tag).
    for line in 0..10u32 {
        let found = tokens.iter().any(|t| t.get_src_line() == line);
        assert!(
            found,
            "deeply nested: no mapping for source line {line}. Tokens: {:?}",
            tokens
                .iter()
                .map(|t| (
                    t.get_dst_line(),
                    t.get_dst_col(),
                    t.get_src_line(),
                    t.get_src_col()
                ))
                .collect::<Vec<_>>()
        );
    }

    // Specifically check some closing tags
    // </h2> is on line 4
    let has_close_h2 = tokens.iter().any(|t| {
        let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 5);
        src.starts_with("</h2>")
    });
    assert!(has_close_h2, "should map </h2>");

    // </article> is on line 7
    let has_close_article = tokens.iter().any(|t| {
        let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 10);
        src.starts_with("</article>")
    });
    assert!(has_close_article, "should map </article>");

    // </section> is on line 8
    let has_close_section = tokens.iter().any(|t| {
        let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 10);
        src.starts_with("</section>")
    });
    assert!(has_close_section, "should map </section>");
}

// ---------------------------------------------------------------
// Complex file: attributes with expressions
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_attributes_with_expressions() {
    let source = r#"---
const cls = "active";
const id = 42;
---
<div class={cls} data-id={id}>
<p>styled</p>
</div>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Frontmatter
    assert!(has_src_line(&tokens, 1), "should map frontmatter line 1");
    assert!(has_src_line(&tokens, 2), "should map frontmatter line 2");
    // Template
    assert!(
        has_src_line(&tokens, 4),
        "should map <div> with attrs on line 4"
    );
    assert!(has_src_line(&tokens, 5), "should map <p> on line 5");
    assert!(has_src_line(&tokens, 6), "should map </div> on line 6");
}

// ---------------------------------------------------------------
// Complex file: HTML comments
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_html_comments() {
    let source = r"<div>
<!-- This is a comment -->
<p>After comment</p>
</div>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // The comment on line 1 should be mapped
    assert!(
        has_src_line(&tokens, 1),
        "HTML comment on line 1 should be mapped"
    );
    // <p> on line 2
    assert!(
        has_src_line(&tokens, 2),
        "element after comment should be mapped"
    );
    // </div> on line 3
    assert!(has_src_line(&tokens, 3), "closing </div> should be mapped");
}

// ---------------------------------------------------------------
// Complex file: fragments
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_fragment_with_map() {
    let source = r#"---
const items = ["a", "b"];
---
<>
{items.map(item => <p>{item}</p>)}
</>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Frontmatter
    assert!(has_src_line(&tokens, 1), "frontmatter mapped");
    // Expression on line 4
    assert!(
        has_src_line(&tokens, 4),
        "map expression on line 4 should be mapped"
    );
}

// ---------------------------------------------------------------
// Complex file: nested map + ternary
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_nested_map_ternary() {
    let source = r#"---
const items = [{name: "a", active: true}, {name: "b", active: false}];
---
<ul>
{items.map(item =>
item.active
  ? <li class="active">{item.name}</li>
  : <li class="inactive">{item.name}</li>
)}
</ul>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let tokens: Vec<_> = sm.get_tokens().collect();

    // Both <li> opening tags should be mapped (lines 6 and 7)
    assert!(
        tokens.iter().any(|t| t.get_src_line() == 6),
        "active <li> on line 6 should be mapped"
    );
    assert!(
        tokens.iter().any(|t| t.get_src_line() == 7),
        "inactive <li> on line 7 should be mapped"
    );

    // Check that the closing </li> tags are mapped
    let close_li_count = tokens
        .iter()
        .filter(|t| {
            let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 5);
            src.starts_with("</li>")
        })
        .count();
    assert!(
        close_li_count >= 2,
        "should map both </li> tags, found {close_li_count}"
    );

    // </ul> on line 9
    let has_close_ul = tokens.iter().any(|t| {
        let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 5);
        src.starts_with("</ul>")
    });
    assert!(has_close_ul, "should map </ul> on line 9");

    // $$render should not map to any <li>
    for t in &tokens {
        let gen_txt = gen_text_at(&result.code, t.get_dst_line(), t.get_dst_col(), 8);
        if gen_txt == "$$render" {
            let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 4);
            assert!(
                !src.starts_with("<li"),
                "$$render should not map to <li> in nested ternary"
            );
        }
    }
}

// ---------------------------------------------------------------
// Complex file: component with children and slots
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_component_with_children() {
    let source = r#"---
import Card from "./Card.astro";
---
<Card title="Hello">
<p>Card body content</p>
<span>More info</span>
</Card>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Component opening on line 3
    assert!(
        has_src_line(&tokens, 3),
        "component <Card> on line 3 should be mapped"
    );
    // Inner elements on lines 4, 5
    assert!(
        has_src_line(&tokens, 4),
        "inner <p> on line 4 should be mapped"
    );
    assert!(
        has_src_line(&tokens, 5),
        "inner <span> on line 5 should be mapped"
    );
}

#[test]
fn test_sourcemap_component_with_named_slots() {
    let source = r#"---
import Layout from "./Layout.astro";
---
<Layout>
<h1 slot="header">Page Title</h1>
<p>Main content</p>
<footer slot="footer">Copyright</footer>
</Layout>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // The header slot (line 4) gets a composed mapping because Phase-2
    // emits AST-node tokens that look up to the `</h1>` Phase-1 token.
    assert!(
        has_src_line(&tokens, 4),
        "header slot on line 4 should be mapped"
    );
    // Default slot content (line 5) is inside template-literal text that
    // the supplement logic can carry forward.
    assert!(
        has_src_line(&tokens, 5),
        "default slot on line 5 should be mapped"
    );
    // The closing </Layout> tag (line 7) should be mapped now that
    // component closing elements emit Phase-1 tokens.
    assert!(
        has_src_line(&tokens, 7),
        "</Layout> on line 7 should be mapped"
    );

    // Footer slot content on line 6 should also be mapped.
    assert!(
        has_src_line(&tokens, 6),
        "footer slot on line 6 should be mapped"
    );
}

// ---------------------------------------------------------------
// Complex file: multiple expression containers
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_multiple_expressions_same_element() {
    let source = r#"---
const first = "Hello";
const last = "World";
const age = 30;
---
<p>{first} {last}, age {age}</p>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();

    // Source line 5: `<p>{first} {last}, age {age}</p>`
    //   <p> at col 0
    //   {first} at col 3
    //   {last} at col 11
    //   {age} at col 23
    //   </p> at col 28
    let line5_tokens: Vec<_> = sm.get_tokens().filter(|t| t.get_src_line() == 5).collect();
    assert!(
        line5_tokens.len() >= 4,
        "line 5 should have mappings for <p>, expressions, and </p>. Found {} tokens",
        line5_tokens.len()
    );

    // Should have a closing </p> mapping
    let has_close_p = line5_tokens.iter().any(|t| t.get_src_col() == 28);
    assert!(
        has_close_p,
        "should map </p> at col 28. Tokens: {line5_tokens:?}"
    );
}

// ---------------------------------------------------------------
// Complex file: void elements (no closing tag)
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_void_elements_no_closing_mapping() {
    // Void elements like <br>, <img>, <input> have no closing tag,
    // so they should still compile and map correctly without errors.
    let source = r#"<div>
<img src="test.png" alt="test" />
<br />
<input type="text" />
<p>after void elements</p>
</div>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // All lines should be mapped
    for line in 0..6u32 {
        assert!(
            has_src_line(&tokens, line),
            "void elements test: line {line} should be mapped"
        );
    }
}

// ---------------------------------------------------------------
// Complex file: logical && rendering
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_logical_and_expression() {
    let source = r"---
const show = true;
const items = [1, 2];
---
<div>
{show && <p>Shown</p>}
{items.length > 0 && <ul>{items.map(i => <li>{i}</li>)}</ul>}
</div>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    assert!(
        has_src_line(&tokens, 5),
        "logical && with <p> on line 5 should be mapped"
    );
    assert!(
        has_src_line(&tokens, 6),
        "logical && with <ul> on line 6 should be mapped"
    );

    // $$render should not point to <p>, <ul>, or <li>
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    for t in sm.get_tokens() {
        let gen_txt = gen_text_at(&result.code, t.get_dst_line(), t.get_dst_col(), 8);
        if gen_txt == "$$render" {
            let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 4);
            assert!(
                !src.starts_with("<p>") && !src.starts_with("<ul>") && !src.starts_with("<li>"),
                "$$render should not map to element tag, got src '{src}'"
            );
        }
    }
}

// ---------------------------------------------------------------
// Complex file: set:html directive
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_set_html_directive() {
    let source = r#"---
const rawHtml = "<strong>bold</strong>";
---
<div set:html={rawHtml} />
<p>After set:html</p>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    assert!(has_src_line(&tokens, 1), "frontmatter should be mapped");
    assert!(
        has_src_line(&tokens, 3),
        "set:html div on line 3 should be mapped"
    );
    assert!(has_src_line(&tokens, 4), "<p> on line 4 should be mapped");
}

// ---------------------------------------------------------------
// Complex file: head + body structure
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_html_head_body() {
    let source = r#"---
const title = "My Page";
---
<html>
<head>
<title>{title}</title>
<meta charset="utf-8" />
</head>
<body>
<h1>{title}</h1>
<p>Welcome</p>
</body>
</html>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Key source lines — note that <html> is the root element; its opening
    // tag may share a generated line with the $$render`` boilerplate, but
    // the supplement logic (with whitespace adjustment) should still produce
    // a mapping.
    assert!(has_src_line(&tokens, 1), "frontmatter should be mapped");
    // <html> on source line 3
    assert!(
        has_src_line(&tokens, 3),
        "source line 3 (<html>) should be mapped"
    );
    assert!(has_src_line(&tokens, 5), "<title> should be mapped");
    assert!(has_src_line(&tokens, 9), "<h1> should be mapped");
    assert!(has_src_line(&tokens, 10), "<p> should be mapped");
}

// ---------------------------------------------------------------
// Complex file: realistic page layout
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_realistic_page() {
    let source = r#"---
import Header from "../components/Header.astro";
import Footer from "../components/Footer.astro";

const title = "Blog Post";
const author = "Alice";
const tags = ["rust", "wasm", "astro"];
const published = true;
---
<html>
<head>
<title>{title}</title>
<meta charset="utf-8" />
</head>
<body>
<Header siteName="My Blog" />
<main>
  <article>
    <h1>{title}</h1>
    <p class="author">By {author}</p>
    {published && <span class="badge">Published</span>}
    <div class="content">
      <p>This is the blog post content.</p>
      <ul class="tags">
        {tags.map(tag => <li class="tag">{tag}</li>)}
      </ul>
    </div>
  </article>
</main>
<Footer year={2024} />
</body>
</html>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let tokens: Vec<_> = sm.get_tokens().collect();
    let tuples = token_tuples(&result.map);

    // --- Frontmatter lines mapped ---
    for line in [4u32, 5, 6, 7] {
        assert!(
            has_src_line(&tuples, line),
            "realistic: frontmatter line {line} should be mapped"
        );
    }

    // --- Template lines mapped ---
    // Key template lines: <title> (11), <Header> (15), <h1> (18),
    // <p> author (19), published && (20), <p> content (22),
    // map expr (24), <Footer> (29).
    // NOTE: <html> (line 9) shares a generated line with $$render boilerplate;
    // it should now be mapped thanks to the whitespace-aware supplement.
    for line in [9, 11, 15, 18, 19, 20, 22, 24, 29] {
        assert!(
            has_src_line(&tuples, line as u32),
            "realistic: template line {line} should be mapped"
        );
    }

    // --- Closing tags should be mapped ---
    // </ul> on line 25
    let has_close_ul = tokens.iter().any(|t| {
        let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 5);
        src.starts_with("</ul>")
    });
    assert!(has_close_ul, "realistic: </ul> should be mapped");

    // </article> on line 27
    let has_close_article = tokens.iter().any(|t| {
        let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 10);
        src.starts_with("</article>")
    });
    assert!(has_close_article, "realistic: </article> should be mapped");

    // </html> on line 31
    let has_close_html = tokens.iter().any(|t| {
        let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 7);
        src.starts_with("</html>")
    });
    assert!(has_close_html, "realistic: </html> should be mapped");

    // --- $$render should never map to an element tag ---
    for t in &tokens {
        let gen_txt = gen_text_at(&result.code, t.get_dst_line(), t.get_dst_col(), 8);
        if gen_txt == "$$render" {
            let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 4);
            assert!(
                !src.starts_with("<li ")
                    && !src.starts_with("<li>")
                    && !src.starts_with("<spa")
                    && !src.starts_with("<p ")
                    && !src.starts_with("<p>"),
                "realistic: $$render should not map to element tag, got '{src}' at src {}:{}",
                t.get_src_line(),
                t.get_src_col()
            );
        }
    }
}

// ---------------------------------------------------------------
// Complex file: multiple components with hydration
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_multiple_components() {
    let source = r#"---
import A from "./A.astro";
import B from "./B.astro";
import C from "./C.astro";
---
<div>
<A />
<B title="hello">
<p>B child</p>
</B>
<C>
<span slot="icon">★</span>
<p>Default slot</p>
</C>
</div>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Each component should be mapped
    assert!(has_src_line(&tokens, 6), "<A /> on line 6 should be mapped");
    assert!(has_src_line(&tokens, 7), "<B> on line 7 should be mapped");
    assert!(
        has_src_line(&tokens, 8),
        "<p> inside B on line 8 should be mapped"
    );
    assert!(has_src_line(&tokens, 10), "<C> on line 10 should be mapped");
    // Slot children are wrapped in $$render`...` by the codegen.
    // The whitespace-aware supplement should carry their Phase 1
    // tokens through even when oxc_codegen adds a leading tab.
    // The closing </B> (line 9) and </C> (line 13) tags should now be
    // mapped thanks to the component closing-element Phase-1 token.
    assert!(has_src_line(&tokens, 9), "</B> on line 9 should be mapped");
    assert!(
        has_src_line(&tokens, 13),
        "</C> on line 13 should be mapped"
    );

    // Slot children on lines 11-12 (`<span slot="icon">★</span>`
    // and `<p>Default slot</p>`) should get fine-grained mappings via
    // text-search supplementation for anchorless quasi-text lines.
    assert!(
        has_src_line(&tokens, 11),
        "slot child on line 11 should be mapped"
    );
    assert!(
        has_src_line(&tokens, 12),
        "slot child on line 12 should be mapped"
    );
}

// ---------------------------------------------------------------
// Complex file: multi-line frontmatter with complex template
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_large_frontmatter_complex_template() {
    let source = r#"---
const a = 1;
const b = 2;
const c = 3;
const d = 4;
const e = 5;
const list = [a, b, c, d, e];
const title = "Numbers";
---
<h1>{title}</h1>
<table>
<thead><tr><th>Value</th><th>Doubled</th></tr></thead>
<tbody>
{list.map(n => <tr><td>{n}</td><td>{n * 2}</td></tr>)}
</tbody>
</table>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Frontmatter lines
    for line in 1..=7 {
        assert!(
            has_src_line(&tokens, line),
            "frontmatter line {line} should be mapped"
        );
    }

    // Template lines
    assert!(has_src_line(&tokens, 9), "<h1> on line 9 should be mapped");
    assert!(
        has_src_line(&tokens, 10),
        "<table> on line 10 should be mapped"
    );
    assert!(
        has_src_line(&tokens, 13),
        "map expression on line 13 should be mapped"
    );

    // Check closing tags
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let has_close_table = sm.get_tokens().any(|t| {
        let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 8);
        src.starts_with("</table>")
    });
    assert!(has_close_table, "should map </table>");

    let has_close_tbody = sm.get_tokens().any(|t| {
        let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 8);
        src.starts_with("</tbody>")
    });
    assert!(has_close_tbody, "should map </tbody>");
}

// ---------------------------------------------------------------
// Complex file: expression returning fragment
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_expression_fragment() {
    let source = r#"---
const items = ["x", "y"];
---
<div>
{items.map(i => <><dt>{i}</dt><dd>value</dd></>)}
</div>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    assert!(
        has_src_line(&tokens, 4),
        "map with fragment on line 4 should be mapped"
    );
}

// ---------------------------------------------------------------
// Complex file: self-closing elements mixed with regular
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_mixed_self_closing_and_regular() {
    let source = r#"<div>
<img src="photo.jpg" alt="A photo" />
<p>Caption below the image</p>
<hr />
<blockquote>
<p>A wise quote</p>
</blockquote>
</div>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let tokens: Vec<_> = sm.get_tokens().collect();

    // Closing tags
    let has_close_blockquote = tokens.iter().any(|t| {
        let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 13);
        src.starts_with("</blockquote>")
    });
    assert!(has_close_blockquote, "should map </blockquote>");

    let has_close_p = tokens.iter().any(|t| {
        let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 4);
        src == "</p>"
    });
    assert!(has_close_p, "should map </p> inside blockquote");
}

// ---------------------------------------------------------------
// Complex file: slot element
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_slot_element() {
    let source = r#"<div class="wrapper">
<slot />
<slot name="sidebar" />
</div>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // <div> line 0
    assert!(has_src_line(&tokens, 0), "<div> should be mapped");
    // slot lines
    assert!(
        has_src_line(&tokens, 1) || has_src_line(&tokens, 2),
        "at least one slot line should be mapped"
    );
}

// ---------------------------------------------------------------
// Complex file: chained method calls with JSX
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_chained_methods() {
    let source = r"---
const data = [1, 2, 3, 4, 5];
---
<ul>
{data.filter(n => n > 2).map(n => <li>{n}</li>)}
</ul>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    assert!(
        has_src_line(&tokens, 4),
        "chained filter/map on line 4 should be mapped"
    );

    // </ul> on line 5
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let has_close_ul = sm.get_tokens().any(|t| {
        let src = src_text_at(source, t.get_src_line(), t.get_src_col(), 5);
        src.starts_with("</ul>")
    });
    assert!(has_close_ul, "should map </ul>");
}

// ---------------------------------------------------------------
// Complex file: arrow function with block body
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_arrow_block_body() {
    let source = r"---
const items = [1, 2, 3];
---
<ul>
{items.map(item => {
const doubled = item * 2;
return <li>{doubled}</li>;
})}
</ul>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // The map expression on line 4
    assert!(has_src_line(&tokens, 4), "map expression should be mapped");
    // The JSX on line 6
    assert!(has_src_line(&tokens, 6), "<li> on line 6 should be mapped");
}

// ---------------------------------------------------------------
// Script tag sourcemap tests
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_hoisted_script_root_no_false_mapping() {
    // Case 1: A hoisted <script> at root level (element_depth == 0)
    // produces no output, so it must NOT create a mapping that falsely
    // points the source <script> to the next generated element (<h1>).
    let source = r#"---
const name = "World";
---
<script>console.log("hello")</script>
<h1>{name}</h1>"#;
    // Source layout (0-indexed):
    //   line 0: ---
    //   line 1: const name = "World";
    //   line 2: ---
    //   line 3: <script>console.log("hello")</script>
    //   line 4: <h1>{name}</h1>
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let tokens: Vec<_> = sm.get_tokens().collect();

    // There should be NO token that maps source line 3 (the <script> line)
    // to generated text that starts with "<h1".
    let gen_lines: Vec<&str> = result.code.lines().collect();
    let false_mapping = tokens.iter().any(|t| {
        if t.get_src_line() == 3 && t.get_src_col() == 0 {
            // Check if the generated text at this token starts with "<h1"
            if let Some(line) = gen_lines.get(t.get_dst_line() as usize) {
                let col = t.get_dst_col() as usize;
                if col < line.len() {
                    return line[col..].starts_with("<h1");
                }
            }
        }
        false
    });
    assert!(
        !false_mapping,
        "hoisted root script should NOT map source <script> to generated <h1>. Tokens: {tokens:?}"
    );

    // The <h1> on source line 4 should still be correctly mapped.
    let has_h1 = tokens
        .iter()
        .any(|t| t.get_src_line() == 4 && t.get_src_col() == 0);
    assert!(has_h1, "should still have mapping for <h1> on line 4");
}

// ---------------------------------------------------------------
// Tests for HTML element spread attribute mapping (Fix 1)
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_html_spread_attribute() {
    // Spread attributes on HTML elements should have sourcemap mappings,
    // just like spread attributes on components do.
    let source = r#"---
const attrs = { id: "main", role: "banner" };
---
<div {...attrs}>content</div>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: const attrs = { id: "main", role: "banner" };
    //   2: ---
    //   3: <div {...attrs}>content</div>

    // The spread attribute on line 3 should be mapped
    assert!(
        has_src_line(&tokens, 3),
        "spread attribute on HTML element on line 3 should be mapped. Tokens: {tokens:?}"
    );

    // The generated code should contain $$spreadAttributes
    assert!(
        result.code.contains("$$spreadAttributes"),
        "generated code should contain $$spreadAttributes. Got:\n{}",
        result.code
    );
}

// ---------------------------------------------------------------
// Column-level sourcemap accuracy tests
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_frontmatter_const_column() {
    // `const x = 42;` starts at column 0 on line 1.
    // The generated output should map it back to column 0 of line 1.
    let source = "---\nconst x = 42;\n---\n<p>{x}</p>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Find tokens on source line 1 (the const declaration)
    let line1_tokens: Vec<_> = tokens.iter().filter(|t| t.2 == 1).collect();
    assert!(
        !line1_tokens.is_empty(),
        "should have tokens mapping to source line 1"
    );

    // At least one token should point to column 0 (the start of `const`)
    let has_col0 = line1_tokens.iter().any(|t| t.3 == 0);
    assert!(
        has_col0,
        "should have a token at source column 0 on line 1 (`const`). Tokens: {line1_tokens:?}"
    );
}

#[test]
fn test_sourcemap_expression_column_offset() {
    // `{name}` starts at column 4 on the template line.
    // We want to verify the column is correct, not just the line.
    let source = "---\nconst name = \"x\";\n---\n<p>{name}</p>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source line 3: `<p>{name}</p>`
    // `{name}` starts at column 3 (0-indexed: <p> is 0,1,2 then { is 3)
    let line3_tokens: Vec<_> = tokens.iter().filter(|t| t.2 == 3).collect();
    assert!(
        !line3_tokens.is_empty(),
        "should have tokens mapping to source line 3"
    );

    // Check that we have a token pointing somewhere in the `{name}` range (col 3-8)
    let has_expr_col = line3_tokens.iter().any(|t| t.3 >= 3 && t.3 <= 8);
    assert!(
        has_expr_col,
        "should have a token in the expression column range (3-8) on line 3. Tokens: {line3_tokens:?}"
    );
}

#[test]
fn test_sourcemap_indented_frontmatter_column() {
    // Frontmatter with indentation — the column should reflect the actual offset.
    let source = "---\n  const y = 99;\n---\n<div>{y}</div>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source line 1: `  const y = 99;` — `const` starts at column 2
    assert!(
        tokens.iter().any(|t| t.2 == 1),
        "should have tokens mapping to source line 1"
    );
}

#[test]
fn test_sourcemap_multiline_frontmatter_columns() {
    // Multiple frontmatter variables each at different columns
    let source = "---\nconst a = 1;\nconst b = 2;\n---\n<p>{a} {b}</p>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Both line 1 and line 2 should have column 0 mappings (each starts with `const`)
    for src_line in [1u32, 2] {
        let line_tokens: Vec<_> = tokens.iter().filter(|t| t.2 == src_line).collect();
        assert!(
            !line_tokens.is_empty(),
            "should have tokens on source line {src_line}"
        );
        let has_col0 = line_tokens.iter().any(|t| t.3 == 0);
        assert!(
            has_col0,
            "source line {src_line} should have a token at column 0. Tokens: {line_tokens:?}"
        );
    }
}

#[test]
fn test_sourcemap_template_column_after_expression() {
    // After an expression like `{x}`, the next HTML text should map to the
    // correct column in the original source.
    let source = "<p>{x} text</p>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // All tokens should be on source line 0
    for t in &tokens {
        assert_eq!(t.2, 0, "all tokens should map to line 0. Token: {t:?}");
    }
}

#[test]
fn test_sourcemap_column_accuracy_on_closing_tag() {
    // `</div>` on a separate line should map to the correct column (0)
    let source = "<div>\n  <p>hi</p>\n</div>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source line 2: `</div>` starts at column 0
    let line2_tokens: Vec<_> = tokens.iter().filter(|t| t.2 == 2).collect();
    assert!(
        !line2_tokens.is_empty(),
        "should have tokens mapping to the closing </div> line. Tokens: {tokens:?}"
    );

    // The mapping should point to column 0 (start of `</div>`)
    let has_col0 = line2_tokens.iter().any(|t| t.3 == 0);
    assert!(
        has_col0,
        "closing </div> should map to column 0 on source line 2. Tokens: {line2_tokens:?}"
    );
}

#[test]
fn test_sourcemap_column_on_attribute_expression() {
    // Verify column-level accuracy for attribute expressions
    let source = "---\nconst href = \"/\";\n---\n<a href={href}>Link</a>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source line 3: `<a href={href}>Link</a>`
    let line3_tokens: Vec<_> = tokens.iter().filter(|t| t.2 == 3).collect();
    assert!(
        !line3_tokens.is_empty(),
        "should have tokens on the template line with attribute expression"
    );

    // Verify a mapping exists near column 0 for the `<a` tag
    let has_tag_start = line3_tokens.iter().any(|t| t.3 <= 2);
    assert!(
        has_tag_start,
        "should have a mapping near column 0 for the <a tag. Tokens: {line3_tokens:?}"
    );
}

// ---------------------------------------------------------------
// Tests for static/boolean HTML attribute mapping (Fix 2)
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_static_html_attribute() {
    // Static string attributes on HTML elements should have sourcemap mappings.
    let source = r#"<input type="text" placeholder="Enter name" />"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: <input type="text" placeholder="Enter name" />

    // The element with static attributes on line 0 should be mapped.
    // With per-attribute mapping, we expect multiple tokens on line 0.
    let line0_tokens: Vec<_> = tokens.iter().filter(|t| t.2 == 0).collect();
    assert!(
        line0_tokens.len() >= 3,
        "should have at least 3 tokens for opening tag + 2 static attributes on line 0, got {}. Tokens: {tokens:?}",
        line0_tokens.len()
    );
}

#[test]
fn test_sourcemap_boolean_html_attribute() {
    // Boolean attributes on HTML elements should have sourcemap mappings.
    let source = r"<input disabled readonly />";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: <input disabled readonly />

    // With per-attribute mapping, we expect tokens for the opening tag
    // plus each boolean attribute.
    let line0_tokens: Vec<_> = tokens.iter().filter(|t| t.2 == 0).collect();
    assert!(
        line0_tokens.len() >= 3,
        "should have at least 3 tokens for opening tag + 2 boolean attributes on line 0, got {}. Tokens: {tokens:?}",
        line0_tokens.len()
    );
}

#[test]
fn test_sourcemap_mixed_static_dynamic_attributes() {
    // Elements with a mix of static and dynamic attributes should map all of them.
    let source = r#"---
const cls = "active";
---
<div id="main" class={cls} hidden>content</div>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: const cls = "active";
    //   2: ---
    //   3: <div id="main" class={cls} hidden>content</div>

    // Should have tokens for: opening tag, id (static), class (dynamic), hidden (boolean)
    let line3_tokens: Vec<_> = tokens.iter().filter(|t| t.2 == 3).collect();
    assert!(
        line3_tokens.len() >= 4,
        "should have at least 4 tokens for opening tag + 3 attributes on line 3, got {}. Tokens: {tokens:?}",
        line3_tokens.len()
    );
}

// ---------------------------------------------------------------
// Tests for switch case label mapping (Fix 3)
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_switch_case_labels_in_jsx() {
    // Individual case labels inside switch statements within JSX-aware
    // arrow function bodies should have sourcemap mappings.
    let source = r"---
const items = [1, 2, 3];
---
<ul>
{items.map(x => {
switch (x) {
case 1:
return <li>one</li>;
case 2:
return <li>two</li>;
default:
return <li>other</li>;
}
})}
</ul>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: const items = [1, 2, 3];
    //   2: ---
    //   3: <ul>
    //   4: {items.map(x => {
    //   5: switch (x) {
    //   6: case 1:
    //   7: return <li>one</li>;
    //   8: case 2:
    //   9: return <li>two</li>;
    //  10: default:
    //  11: return <li>other</li>;
    //  12: }
    //  13: })}
    //  14: </ul>

    // The switch on line 5 should be mapped
    assert!(
        has_src_line(&tokens, 5),
        "switch statement on line 5 should be mapped. Tokens: {tokens:?}"
    );

    // case 1: on line 6 should be mapped
    assert!(
        has_src_line(&tokens, 6),
        "case 1 on line 6 should be mapped. Tokens: {tokens:?}"
    );

    // case 2: on line 8 should be mapped
    assert!(
        has_src_line(&tokens, 8),
        "case 2 on line 8 should be mapped. Tokens: {tokens:?}"
    );

    // default: on line 10 should be mapped
    assert!(
        has_src_line(&tokens, 10),
        "default case on line 10 should be mapped. Tokens: {tokens:?}"
    );
}

// ---------------------------------------------------------------
// Tests for dynamic slot expression mapping (Fix 4)
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_dynamic_slot_expression() {
    // Dynamic slot expressions (slot={expr}) on component children
    // should have sourcemap mappings.
    let source = r#"---
import Layout from './Layout.astro';
const slotName = "header";
---
<Layout>
<div slot={slotName}>dynamic slot content</div>
</Layout>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: import Layout ...
    //   2: const slotName = "header";
    //   3: ---
    //   4: <Layout>
    //   5: <div slot={slotName}>dynamic slot content</div>
    //   6: </Layout>

    // The div with dynamic slot on line 5 should be mapped
    assert!(
        has_src_line(&tokens, 5),
        "dynamic slot element on line 5 should be mapped. Tokens: {tokens:?}"
    );
}

#[test]
fn test_sourcemap_hoisted_script_nested_maps_to_render_script() {
    // Case 2: A hoisted <script> nested inside an element (element_depth > 0)
    // produces $$renderScript(...) output and should be mapped.
    let source = r#"<main>
<script>console.log("hello")</script>
<p>Content</p>
</main>"#;
    // Source layout (no indentation in raw string):
    //   line 0: <main>
    //   line 1: <script>console.log("hello")</script>
    //   line 2: <p>Content</p>
    //   line 3: </main>
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let tokens: Vec<_> = sm.get_tokens().collect();

    // Source line 1, col 0 = "<script>" — should map to generated code
    // containing "$$renderScript".
    let gen_lines: Vec<&str> = result.code.lines().collect();
    let maps_to_render_script = tokens.iter().any(|t| {
        if t.get_src_line() == 1 && t.get_src_col() == 0
            && let Some(line) = gen_lines.get(t.get_dst_line() as usize) {
                return line.contains("$$renderScript");
            }
        false
    });
    assert!(
        maps_to_render_script,
        "nested hoisted script should map to $$renderScript. Tokens: {tokens:?}"
    );
}

#[test]
fn test_sourcemap_script_in_conditional_expression() {
    // Case 3: A <script> inside a conditional expression.
    let source = r#"<main>{true && <script>console.log("hello")</script>}</main>"#;
    // Source layout (all on line 0):
    //   col 0: <main>
    //   col 15: <script>
    //   col 44: </script>
    //   col 53: }
    //   col 54: </main>
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let tokens: Vec<_> = sm.get_tokens().collect();

    // The generated code should contain $$renderScript and the source
    // position should map correctly.
    let gen_lines: Vec<&str> = result.code.lines().collect();
    let has_render_script = gen_lines.iter().any(|l| l.contains("$$renderScript"));
    assert!(
        has_render_script,
        "script in conditional should produce $$renderScript"
    );

    // The <main> opening tag should be mapped.
    let has_main = tokens
        .iter()
        .any(|t| t.get_src_line() == 0 && t.get_src_col() == 0);
    assert!(has_main, "should have mapping for <main> at 0:0");
}

#[test]
fn test_sourcemap_inline_script_has_mappings() {
    // Case 4: An inline <script is:inline> should preserve its content
    // literally and its sourcemap mappings should survive composition,
    // even though oxc_codegen escapes </script> as <\/script>.
    let source = r#"<main>
<script is:inline>console.log("inline")</script>
<p>After inline</p>
</main>"#;
    // Source layout (no indentation in raw string):
    //   line 0: <main>
    //   line 1: <script is:inline>console.log("inline")</script>
    //           col 0 = <script
    //           col 18 = console.log("inline")
    //           col 39 = </script>
    //   line 2: <p>After inline</p>
    //   line 3: </main>
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map).unwrap();
    let tokens: Vec<_> = sm.get_tokens().collect();

    // The generated code should contain the inline script content literally.
    assert!(
        result.code.contains("console.log("),
        "inline script content should appear in generated code"
    );

    // There should be at least one mapping for source line 1 (the script line).
    assert!(
        tokens.iter().any(|t| t.get_src_line() == 1),
        "inline script line should have at least one sourcemap token. All tokens: {tokens:?}"
    );

    // Specifically, the <script opening tag at col 0 should be mapped.
    let has_script_open = tokens
        .iter()
        .any(|t| t.get_src_line() == 1 && t.get_src_col() == 0);
    assert!(
        has_script_open,
        "should have mapping for <script is:inline> opening tag at 1:0. Tokens: {tokens:?}"
    );

    // The text content (console.log) should be mapped. It starts at col 18.
    let has_text_content = tokens
        .iter()
        .any(|t| t.get_src_line() == 1 && t.get_src_col() == 18);
    assert!(
        has_text_content,
        "should have mapping for inline script text content at 1:18. Tokens: {tokens:?}"
    );

    // The closing </script> tag at col 39 should be mapped.
    let has_close_script = tokens
        .iter()
        .any(|t| t.get_src_line() == 1 && t.get_src_col() == 39);
    assert!(
        has_close_script,
        "should have mapping for </script> closing tag at 1:39. Tokens: {tokens:?}"
    );

    // Elements after the inline script should also be mapped.
    let has_p = tokens
        .iter()
        .any(|t| t.get_src_line() == 2 && t.get_src_col() == 0);
    assert!(has_p, "should have mapping for <p> on line 2");
}

#[test]
fn test_sourcemap_multiline_frontmatter_per_line_mapping() {
    // Multi-line frontmatter: each source line of the statement should map
    // back to its own line, not all collapse to the statement-start line.
    let source = r"---
console.log(
	'Hello from Astro!',
)();
---

<h1>Astro</h1>";
    let result = compile_astro_with_sourcemap(source);
    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map)
        .expect("should parse as valid sourcemap");
    let tokens: Vec<_> = sm.get_tokens().collect();

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: console.log(
    //   2: 	'Hello from Astro!',
    //   3: )();
    //   4: ---
    //   5: (blank)
    //   6: <h1>Astro</h1>

    // Line 1: console.log(  — should have at least one mapping
    let has_line1 = tokens.iter().any(|t| t.get_src_line() == 1);
    assert!(
        has_line1,
        "should have mapping for console.log( on source line 1. Tokens: {tokens:?}"
    );

    // Line 2: the string literal — should have at least one mapping
    let has_line2 = tokens.iter().any(|t| t.get_src_line() == 2);
    assert!(
        has_line2,
        "should have mapping for string literal on source line 2. Tokens: {tokens:?}"
    );

    // Line 3: )(); — should have at least one mapping
    let has_line3 = tokens.iter().any(|t| t.get_src_line() == 3);
    assert!(
        has_line3,
        "should have mapping for )(); on source line 3. Tokens: {tokens:?}"
    );

    // Critically, NOT everything should map to line 1.
    // At least one of lines 2 or 3 must have a distinct mapping.
    let unique_src_lines: std::collections::HashSet<u32> = tokens
        .iter()
        .filter(|t| t.get_src_line() >= 1 && t.get_src_line() <= 3)
        .map(oxc_sourcemap::Token::get_src_line)
        .collect();
    assert!(
        unique_src_lines.len() >= 2,
        "multi-line frontmatter should map to multiple distinct source lines, \
         not all collapse to one. Unique lines: {unique_src_lines:?}"
    );
}

#[test]
fn test_sourcemap_multiline_frontmatter_with_typescript() {
    // Ensure TypeScript in multi-line frontmatter is stripped by Phase 2,
    // and sourcemap still has per-line mappings.
    let source = r"---
interface Props {
title: string;
}
const props: Props = Astro.props;
const msg: string = `Hello ${
props.title
}`;
---
<h1>{msg}</h1>";
    let result = compile_astro_with_sourcemap(source);

    // TypeScript should be stripped
    assert!(
        !result.code.contains("interface Props"),
        "interface should be stripped from output"
    );
    assert!(
        !result.code.contains(": Props"),
        "type annotation ': Props' should be stripped"
    );
    assert!(
        !result.code.contains(": string"),
        "type annotation ': string' should be stripped"
    );

    let sm = oxc_sourcemap::SourceMap::from_json_string(&result.map)
        .expect("should parse as valid sourcemap");
    let tokens: Vec<_> = sm.get_tokens().collect();

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: interface Props {
    //   2:   title: string;
    //   3: }
    //   4: const props: Props = Astro.props;
    //   5: const msg: string = `Hello ${
    //   6:   props.title
    //   7: }`;
    //   8: ---
    //   9: <h1>{msg}</h1>

    // `const props` (line 4) should be mapped
    let has_const_props = tokens.iter().any(|t| t.get_src_line() == 4);
    assert!(
        has_const_props,
        "should have mapping for const props on source line 4. Tokens: {tokens:?}"
    );

    // `const msg` (line 5) should be mapped
    let has_const_msg = tokens.iter().any(|t| t.get_src_line() == 5);
    assert!(
        has_const_msg,
        "should have mapping for const msg on source line 5. Tokens: {tokens:?}"
    );

    // Template element should also be mapped
    let has_h1 = tokens.iter().any(|t| t.get_src_line() == 9);
    assert!(
        has_h1,
        "should have mapping for <h1> on source line 9. Tokens: {tokens:?}"
    );
}

// ---------------------------------------------------------------
// Tests for slot sourcemap emissions (slots.rs)
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_named_slots_have_mappings() {
    // Named slots printed via print_component_slots should produce
    // sourcemap tokens that map back to the slotted element lines.
    let source = r#"---
import Card from './Card.astro';
---
<Card>
<h2 slot="title">My Title</h2>
<p>Default content</p>
</Card>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: import Card from './Card.astro';
    //   2: ---
    //   3: <Card>
    //   4: <h2 slot="title">My Title</h2>
    //   5: <p>Default content</p>
    //   6: </Card>

    // <Card> opening (line 3) should be mapped
    assert!(
        has_src_line(&tokens, 3),
        "<Card> on line 3 should be mapped. Tokens: {tokens:?}"
    );

    // The slotted element on line 4 should be mapped
    assert!(
        has_src_line(&tokens, 4),
        "slotted <h2> on line 4 should be mapped. Tokens: {tokens:?}"
    );

    // Default content on line 5 should be mapped
    assert!(
        has_src_line(&tokens, 5),
        "default slot <p> on line 5 should be mapped. Tokens: {tokens:?}"
    );
}

#[test]
fn test_sourcemap_conditional_slot_branch() {
    // Conditional slot branches (ternary with different slot attributes)
    // should have mappings for the conditional expression and JSX elements.
    let source = r#"---
import Comp from './Comp.astro';
const showA = true;
---
<Comp>
{showA ? <div slot="a">A</div> : <div slot="b">B</div>}
</Comp>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: import Comp ...
    //   2: const showA = true;
    //   3: ---
    //   4: <Comp>
    //   5: {showA ? <div slot="a">A</div> : <div slot="b">B</div>}
    //   6: </Comp>

    // The conditional expression on line 5 should have mappings
    assert!(
        has_src_line(&tokens, 5),
        "conditional slot expression on line 5 should be mapped. Tokens: {tokens:?}"
    );
}

// ---------------------------------------------------------------
// Tests for print_jsx_aware_statement sourcemap emissions
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_jsx_aware_block_body_statements() {
    // Statements inside arrow function block bodies used in JSX expressions
    // (print_jsx_aware_statement) should have sourcemap mappings.
    let source = r"---
const items = [1, 2, 3];
---
<ul>
{items.map(item => {
const doubled = item * 2;
return <li>{doubled}</li>;
})}
</ul>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: const items = [1, 2, 3];
    //   2: ---
    //   3: <ul>
    //   4: {items.map(item => {
    //   5: const doubled = item * 2;
    //   6: return <li>{doubled}</li>;
    //   7: })}
    //   8: </ul>

    // The `const doubled` on line 5 should be mapped
    assert!(
        has_src_line(&tokens, 5),
        "const doubled on line 5 should be mapped. Tokens: {tokens:?}"
    );

    // The `return` on line 6 should be mapped
    assert!(
        has_src_line(&tokens, 6),
        "return statement on line 6 should be mapped. Tokens: {tokens:?}"
    );
}

// ---------------------------------------------------------------
// Tests for slot element closing tag mapping
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_slot_element_closing_tag() {
    // The closing </slot> tag should get a sourcemap mapping, similar
    // to regular HTML element closing tags.
    let source = r"<div>
<slot>
<p>fallback</p>
</slot>
</div>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: <div>
    //   1: <slot>
    //   2: <p>fallback</p>
    //   3: </slot>
    //   4: </div>

    // <slot> opening (line 1) should be mapped
    assert!(
        has_src_line(&tokens, 1),
        "<slot> on line 1 should be mapped. Tokens: {tokens:?}"
    );

    // </slot> closing (line 3) should be mapped
    assert!(
        has_src_line(&tokens, 3),
        "</slot> on line 3 should be mapped. Tokens: {tokens:?}"
    );
}

// ---------------------------------------------------------------
// Tests for attribute sourcemap emissions
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_dynamic_html_attribute() {
    // Dynamic attributes on HTML elements (class={expr}) should have
    // sourcemap mappings pointing back to the attribute in the source.
    let source = r#"---
const cls = "active";
---
<div class={cls}>content</div>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: const cls = "active";
    //   2: ---
    //   3: <div class={cls}>content</div>

    // The dynamic attribute on line 3 should be mapped
    assert!(
        has_src_line(&tokens, 3),
        "dynamic attribute on line 3 should be mapped. Tokens: {tokens:?}"
    );
}

#[test]
fn test_sourcemap_component_attribute_expression() {
    // Component attributes with expression values should have mappings.
    let source = r#"---
import Button from './Button.astro';
const label = "Click";
---
<Button text={label} disabled />"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: import Button ...
    //   2: const label = "Click";
    //   3: ---
    //   4: <Button text={label} disabled />

    // The component with attributes on line 4 should be mapped
    assert!(
        has_src_line(&tokens, 4),
        "component attributes on line 4 should be mapped. Tokens: {tokens:?}"
    );
}

#[test]
fn test_sourcemap_component_spread_attribute() {
    // Spread attributes on components should have mappings.
    let source = r#"---
import Card from './Card.astro';
const props = { title: "Hello", size: "lg" };
---
<Card {...props} />"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: import Card ...
    //   2: const props = { ... };
    //   3: ---
    //   4: <Card {...props} />

    // The spread attribute on line 4 should be mapped
    assert!(
        has_src_line(&tokens, 4),
        "spread attribute on line 4 should be mapped. Tokens: {tokens:?}"
    );
}

// ---------------------------------------------------------------
// Tests for directive sourcemap emissions
// ---------------------------------------------------------------

#[test]
fn test_sourcemap_set_html_on_html_element() {
    // set:html directive on an HTML element should have a sourcemap
    // mapping pointing back to the directive in the source.
    let source = r#"---
const content = "<em>bold</em>";
---
<div set:html={content} />"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: const content = "<em>bold</em>";
    //   2: ---
    //   3: <div set:html={content} />

    // The set:html directive on line 3 should be mapped
    assert!(
        has_src_line(&tokens, 3),
        "set:html on line 3 should be mapped. Tokens: {tokens:?}"
    );

    // Verify the source text at the mapped position points to "set:html" or "<div"
    assert!(
        tokens.iter().any(|t| t.2 == 3),
        "should have tokens pointing to line 3"
    );
}

#[test]
fn test_sourcemap_set_text_on_html_element() {
    // set:text directive on an HTML element should have a sourcemap mapping.
    let source = r#"---
const msg = "hello";
---
<span set:text={msg} />"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: const msg = "hello";
    //   2: ---
    //   3: <span set:text={msg} />

    assert!(
        has_src_line(&tokens, 3),
        "set:text on line 3 should be mapped. Tokens: {tokens:?}"
    );
}

#[test]
fn test_sourcemap_transition_name_on_html_element() {
    // transition:name on an HTML element should have a sourcemap mapping.
    let source = r#"<div transition:name="fade">content</div>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: <div transition:name="fade">content</div>

    // The transition:name on line 0 should be mapped
    assert!(
        has_src_line(&tokens, 0),
        "transition:name on line 0 should be mapped. Tokens: {tokens:?}"
    );

    // The generated code should contain $$renderTransition
    assert!(
        result.code.contains("$$renderTransition"),
        "generated code should contain $$renderTransition. Got:\n{}",
        result.code
    );
}

#[test]
fn test_sourcemap_transition_persist_on_html_element() {
    // transition:persist on an HTML element should have a sourcemap mapping.
    let source = r"<div transition:persist>content</div>";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // The transition:persist on line 0 should be mapped
    assert!(
        has_src_line(&tokens, 0),
        "transition:persist on line 0 should be mapped. Tokens: {tokens:?}"
    );

    // The generated code should contain data-astro-transition-persist
    assert!(
        result.code.contains("data-astro-transition-persist"),
        "generated code should contain data-astro-transition-persist. Got:\n{}",
        result.code
    );
}

#[test]
fn test_sourcemap_class_list_merged_with_static() {
    // class:list merged with a static class attribute should have a sourcemap mapping.
    let source = r#"---
const isActive = true;
---
<div class="base" class:list={[isActive && "active"]}>content</div>"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: const isActive = true;
    //   2: ---
    //   3: <div class="base" class:list={[isActive && "active"]}>content</div>

    // The merged class:list on line 3 should be mapped
    assert!(
        has_src_line(&tokens, 3),
        "class:list on line 3 should be mapped. Tokens: {tokens:?}"
    );

    // The generated code should use $$addAttribute for the merged class
    assert!(
        result.code.contains("$$addAttribute"),
        "generated code should contain $$addAttribute for merged class:list. Got:\n{}",
        result.code
    );
}

#[test]
fn test_sourcemap_set_html_on_component() {
    // set:html on a component should have a sourcemap mapping.
    let source = r#"---
import Card from './Card.astro';
const html = "<p>content</p>";
---
<Card set:html={html} />"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: import Card ...
    //   2: const html = "<p>content</p>";
    //   3: ---
    //   4: <Card set:html={html} />

    // The component with set:html on line 4 should be mapped
    assert!(
        has_src_line(&tokens, 4),
        "set:html on component on line 4 should be mapped. Tokens: {tokens:?}"
    );

    // The generated code should contain $$unescapeHTML for the expression
    assert!(
        result.code.contains("$$unescapeHTML"),
        "generated code should contain $$unescapeHTML for set:html expression. Got:\n{}",
        result.code
    );
}

#[test]
fn test_sourcemap_transition_name_on_component() {
    // transition:name on a component should have a sourcemap mapping.
    let source = r#"---
import Widget from './Widget.astro';
---
<Widget transition:name="slide" />"#;
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: import Widget ...
    //   2: ---
    //   3: <Widget transition:name="slide" />

    // The component with transition:name on line 3 should be mapped
    assert!(
        has_src_line(&tokens, 3),
        "transition:name on component on line 3 should be mapped. Tokens: {tokens:?}"
    );

    // The generated code should contain $$renderTransition
    assert!(
        result.code.contains("$$renderTransition"),
        "generated code should contain $$renderTransition. Got:\n{}",
        result.code
    );
}

#[test]
fn test_sourcemap_transition_persist_on_component() {
    // transition:persist on a component should have a sourcemap mapping.
    let source = r"---
import Counter from './Counter.astro';
---
<Counter transition:persist />";
    let result = compile_astro_with_sourcemap(source);
    assert_tokens_in_bounds(source, &result);
    let tokens = token_tuples(&result.map);

    // Source layout (0-indexed lines):
    //   0: ---
    //   1: import Counter ...
    //   2: ---
    //   3: <Counter transition:persist />

    // The component with transition:persist on line 3 should be mapped
    assert!(
        has_src_line(&tokens, 3),
        "transition:persist on component on line 3 should be mapped. Tokens: {tokens:?}"
    );

    // The generated code should contain data-astro-transition-persist
    assert!(
        result.code.contains("data-astro-transition-persist"),
        "generated code should contain data-astro-transition-persist. Got:\n{}",
        result.code
    );
}
