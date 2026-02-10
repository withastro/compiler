//! String escaping and HTML entity decoding utilities.
//!
//! Pure functions with no dependency on printer state. Used by the printer
//! to safely embed content inside JavaScript template literals, HTML attributes,
//! and quoted strings.

use cow_utils::CowUtils;
use oxc_syntax::xml_entities::XML_ENTITIES;

/// Escape a string for safe embedding inside a JavaScript template literal.
///
/// Escapes backticks, `${` sequences, and backslashes.
pub fn escape_template_literal(s: &str) -> String {
    let mut result = String::with_capacity(s.len());
    let mut chars = s.chars().peekable();

    while let Some(c) = chars.next() {
        match c {
            '`' => result.push_str("\\`"),
            '$' if chars.peek() == Some(&'{') => {
                result.push_str("\\$");
            }
            '\\' => result.push_str("\\\\"),
            _ => result.push(c),
        }
    }

    result
}

/// Escape double quotes for embedding inside a `"..."` string.
///
/// Only escapes `"` — backslashes are not escaped because the inputs are
/// HTML attribute values or AST string literals, which don't contain
/// escape sequences. Matches the Go compiler's `escapeDoubleQuote`.
pub fn escape_double_quotes(s: &str) -> String {
    s.cow_replace('"', "\\\"").into_owned()
}

/// Escape single quotes for embedding inside a `'...'` string.
///
/// Only escapes `'` — see [`escape_double_quotes`] for rationale on
/// why backslashes are not escaped.
pub fn escape_single_quote(s: &str) -> String {
    s.cow_replace('\'', "\\'").into_owned()
}

/// Escape a string for use as an HTML attribute value inside a template literal.
///
/// Escapes template literal syntax (`` ` `` and `${`), HTML special characters
/// (`"`, `<`, `>`), and ampersands that are not part of valid HTML entities.
pub fn escape_html_attribute(s: &str) -> String {
    // Escape template literal syntax since we're inside a template literal
    let s = s.cow_replace('`', "\\`");
    let s = s.cow_replace("${", "\\${");
    // Escape HTML entities, but preserve valid HTML entities
    let s = escape_ampersands(&s);
    let s = s.cow_replace('"', "&quot;");
    let s = s.cow_replace('<', "&lt;");
    s.cow_replace('>', "&gt;").into_owned()
}

/// Escape ampersands, but preserve valid HTML entities like `&#x22;` or `&quot;`.
fn escape_ampersands(s: &str) -> std::borrow::Cow<'_, str> {
    if !s.contains('&') {
        return std::borrow::Cow::Borrowed(s);
    }

    let mut result = String::with_capacity(s.len());
    let mut i = 0;
    let bytes = s.as_bytes();

    while i < bytes.len() {
        if bytes[i] == b'&' {
            // Check if this is part of a valid HTML entity
            if is_html_entity_start(&s[i..]) {
                // Keep the & as-is (part of an entity)
                result.push('&');
            } else {
                // Escape the &
                result.push_str("&amp;");
            }
            i += 1;
        } else {
            // Advance by one character (may be multi-byte UTF-8)
            let c = s[i..].chars().next().unwrap();
            result.push(c);
            i += c.len_utf8();
        }
    }

    std::borrow::Cow::Owned(result)
}

/// Decode HTML entities in a string.
///
/// Handles numeric entities like `&#x3C;` (hex) and `&#60;` (decimal),
/// and common named entities like `&lt;`, `&gt;`, `&amp;`, `&quot;`, `&apos;`.
pub fn decode_html_entities(s: &str) -> String {
    let mut result = String::with_capacity(s.len());
    let mut chars = s.chars().peekable();

    while let Some(c) = chars.next() {
        if c == '&' {
            // Try to parse an entity
            let mut entity = String::new();
            entity.push(c);

            while let Some(&next) = chars.peek() {
                entity.push(next);
                chars.next();
                if next == ';' {
                    break;
                }
                // Stop if we hit a non-entity character
                if !next.is_ascii_alphanumeric() && next != '#' {
                    break;
                }
            }

            // Try to decode the entity
            if let Some(decoded) = decode_entity(&entity) {
                result.push(decoded);
            } else {
                // Not a valid entity, keep as-is
                result.push_str(&entity);
            }
        } else {
            result.push(c);
        }
    }

    result
}

/// Decode a single HTML entity like `&lt;` → `<` or `&#x3C;` → `<`.
///
/// Uses [`oxc_syntax::xml_entities::XML_ENTITIES`] for named entity lookup,
/// which covers all 252 standard HTML/XML entities.
fn decode_entity(entity: &str) -> Option<char> {
    if !entity.starts_with('&') || !entity.ends_with(';') {
        return None;
    }

    let inner = &entity[1..entity.len() - 1];

    // Numeric entities
    if let Some(hex) = inner
        .strip_prefix("#x")
        .or_else(|| inner.strip_prefix("#X"))
    {
        return u32::from_str_radix(hex, 16).ok().and_then(char::from_u32);
    }
    if let Some(dec) = inner.strip_prefix('#') {
        return dec.parse::<u32>().ok().and_then(char::from_u32);
    }

    // Named entities — look up in the comprehensive XML_ENTITIES map
    XML_ENTITIES.get(inner).copied()
}

/// Check if a string starting with `&` is a valid HTML entity start.
///
/// Used by [`escape_ampersands`] to distinguish standalone `&` characters
/// (which should be escaped to `&amp;`) from `&` that begins something that
/// *looks like* an entity reference (which should be preserved as-is).
///
/// This check is intentionally permissive: `&word` (without a trailing `;`)
/// is treated as a potential entity to avoid over-escaping content like URL
/// query parameters (e.g. `&q=75`).
fn is_html_entity_start(s: &str) -> bool {
    let Some(rest) = s.strip_prefix('&') else {
        return false;
    };

    if rest.is_empty() {
        return false;
    }

    // Numeric entity: &#x... (hex) or &#... (decimal)
    if let Some(after_hash) = rest.strip_prefix('#') {
        // Hex: &#x followed by hex digits
        if let Some(hex_part) = after_hash
            .strip_prefix('x')
            .or_else(|| after_hash.strip_prefix('X'))
        {
            return hex_part
                .chars()
                .next()
                .is_some_and(|c| c.is_ascii_hexdigit());
        }
        // Decimal: &# followed by digits
        return after_hash
            .chars()
            .next()
            .is_some_and(|c| c.is_ascii_digit());
    }

    // Named entity: & followed by alphanumeric (common entities like &quot;, &amp;, etc.)
    // Intentionally permissive — also matches URL query params like &q=75
    rest.chars()
        .next()
        .is_some_and(|c| c.is_ascii_alphanumeric())
}

#[cfg(test)]
mod tests {
    use super::*;

    // ---- escape_template_literal ----

    #[test]
    fn template_literal_no_special_chars() {
        assert_eq!(escape_template_literal("hello world"), "hello world");
    }

    #[test]
    fn template_literal_escapes_backtick() {
        assert_eq!(escape_template_literal("a`b"), "a\\`b");
        assert_eq!(escape_template_literal("``"), "\\`\\`");
    }

    #[test]
    fn template_literal_escapes_dollar_brace() {
        assert_eq!(escape_template_literal("${foo}"), "\\${foo}");
        assert_eq!(escape_template_literal("a${b}c"), "a\\${b}c");
    }

    #[test]
    fn template_literal_dollar_without_brace_unchanged() {
        assert_eq!(escape_template_literal("$100"), "$100");
        assert_eq!(escape_template_literal("a$b"), "a$b");
    }

    #[test]
    fn template_literal_escapes_backslash() {
        assert_eq!(escape_template_literal("a\\b"), "a\\\\b");
        assert_eq!(escape_template_literal("\\"), "\\\\");
    }

    #[test]
    fn template_literal_combined_specials() {
        // All three specials in one string
        assert_eq!(escape_template_literal("`${\\"), "\\`\\${\\\\");
    }

    #[test]
    fn template_literal_empty_string() {
        assert_eq!(escape_template_literal(""), "");
    }

    #[test]
    fn template_literal_unicode() {
        assert_eq!(escape_template_literal("héllo 🌍"), "héllo 🌍");
    }

    #[test]
    fn template_literal_nested_template() {
        // Simulates content that looks like nested template literals
        assert_eq!(
            escape_template_literal("html`<div>${x}</div>`"),
            "html\\`<div>\\${x}</div>\\`"
        );
    }

    // ---- escape_double_quotes ----

    #[test]
    fn double_quotes_basic() {
        assert_eq!(escape_double_quotes("hello"), "hello");
    }

    #[test]
    fn double_quotes_escapes_quotes() {
        assert_eq!(escape_double_quotes(r#"say "hi""#), r#"say \"hi\""#);
    }

    #[test]
    fn double_quotes_empty() {
        assert_eq!(escape_double_quotes(""), "");
    }

    #[test]
    fn double_quotes_only_quotes() {
        assert_eq!(escape_double_quotes(r#""""#), r#"\"\""#);
    }

    #[test]
    fn double_quotes_preserves_backslash() {
        // Per the doc: backslashes are NOT escaped
        assert_eq!(escape_double_quotes("a\\b"), "a\\b");
    }

    // ---- escape_single_quote ----

    #[test]
    fn single_quote_basic() {
        assert_eq!(escape_single_quote("hello"), "hello");
    }

    #[test]
    fn single_quote_escapes_quotes() {
        assert_eq!(escape_single_quote("it's"), "it\\'s");
    }

    #[test]
    fn single_quote_empty() {
        assert_eq!(escape_single_quote(""), "");
    }

    #[test]
    fn single_quote_preserves_backslash() {
        assert_eq!(escape_single_quote("a\\b"), "a\\b");
    }

    // ---- escape_html_attribute ----

    #[test]
    fn html_attr_no_special_chars() {
        assert_eq!(escape_html_attribute("hello"), "hello");
    }

    #[test]
    fn html_attr_escapes_backtick() {
        assert_eq!(escape_html_attribute("a`b"), "a\\`b");
    }

    #[test]
    fn html_attr_escapes_dollar_brace() {
        assert_eq!(escape_html_attribute("${x}"), "\\${x}");
    }

    #[test]
    fn html_attr_escapes_double_quote() {
        assert_eq!(escape_html_attribute("a\"b"), "a&quot;b");
    }

    #[test]
    fn html_attr_escapes_angle_brackets() {
        assert_eq!(escape_html_attribute("<script>"), "&lt;script&gt;");
    }

    #[test]
    fn html_attr_escapes_bare_ampersand() {
        assert_eq!(escape_html_attribute("a & b"), "a &amp; b");
    }

    #[test]
    fn html_attr_preserves_valid_entity() {
        assert_eq!(escape_html_attribute("&lt;"), "&lt;");
        assert_eq!(escape_html_attribute("&amp;"), "&amp;");
        assert_eq!(escape_html_attribute("&#x22;"), "&#x22;");
        assert_eq!(escape_html_attribute("&#60;"), "&#60;");
    }

    #[test]
    fn html_attr_combined_xss_attempt() {
        // A typical XSS vector in an attribute context
        assert_eq!(
            escape_html_attribute(r#"" onload="alert(1)"#),
            "&quot; onload=&quot;alert(1)"
        );
    }

    #[test]
    fn html_attr_template_literal_injection() {
        // Attempting to break out of template literal in attribute
        assert_eq!(escape_html_attribute("`+alert(1)+`"), "\\`+alert(1)+\\`");
    }

    #[test]
    fn html_attr_expression_injection() {
        assert_eq!(escape_html_attribute("${alert(1)}"), "\\${alert(1)}");
    }

    // ---- escape_ampersands (tested through escape_html_attribute) ----

    #[test]
    fn ampersands_no_ampersand() {
        // No & at all — should borrow, not allocate
        let result = escape_ampersands("hello world");
        assert_eq!(&*result, "hello world");
        assert!(matches!(result, std::borrow::Cow::Borrowed(_)));
    }

    #[test]
    fn ampersands_bare_ampersand() {
        assert_eq!(&*escape_ampersands("a & b"), "a &amp; b");
    }

    #[test]
    fn ampersands_trailing_ampersand() {
        assert_eq!(&*escape_ampersands("a&"), "a&amp;");
    }

    #[test]
    fn ampersands_preserves_named_entity() {
        assert_eq!(&*escape_ampersands("&quot;"), "&quot;");
        assert_eq!(&*escape_ampersands("&lt;"), "&lt;");
        assert_eq!(&*escape_ampersands("&gt;"), "&gt;");
        assert_eq!(&*escape_ampersands("&amp;"), "&amp;");
    }

    #[test]
    fn ampersands_preserves_hex_entity() {
        assert_eq!(&*escape_ampersands("&#x3C;"), "&#x3C;");
        assert_eq!(&*escape_ampersands("&#X3C;"), "&#X3C;");
    }

    #[test]
    fn ampersands_preserves_decimal_entity() {
        assert_eq!(&*escape_ampersands("&#60;"), "&#60;");
    }

    #[test]
    fn ampersands_url_query_params() {
        // &q=75 starts with & followed by alphanumeric — treated as potential entity
        assert_eq!(
            &*escape_ampersands("image.jpg?w=100&q=75"),
            "image.jpg?w=100&q=75"
        );
    }

    #[test]
    fn ampersands_mixed() {
        assert_eq!(
            &*escape_ampersands("a & b &lt; c &amp; d"),
            "a &amp; b &lt; c &amp; d"
        );
    }

    #[test]
    fn ampersands_unicode_after_amp() {
        // & followed by non-ASCII — not an entity start
        assert_eq!(&*escape_ampersands("&é"), "&amp;é");
    }

    // ---- is_html_entity_start ----

    #[test]
    fn entity_start_named() {
        assert!(is_html_entity_start("&lt;"));
        assert!(is_html_entity_start("&amp;"));
        assert!(is_html_entity_start("&quot;"));
    }

    #[test]
    fn entity_start_hex() {
        assert!(is_html_entity_start("&#x3C;"));
        assert!(is_html_entity_start("&#X3C;"));
    }

    #[test]
    fn entity_start_decimal() {
        assert!(is_html_entity_start("&#60;"));
    }

    #[test]
    fn entity_start_bare_ampersand() {
        assert!(!is_html_entity_start("& "));
        assert!(!is_html_entity_start("&"));
    }

    #[test]
    fn entity_start_ampersand_space() {
        assert!(!is_html_entity_start("& foo"));
    }

    #[test]
    fn entity_start_not_ampersand() {
        assert!(!is_html_entity_start("hello"));
        assert!(!is_html_entity_start(""));
    }

    #[test]
    fn entity_start_hash_no_digit() {
        // &#z is not valid (z is not a digit)
        assert!(!is_html_entity_start("&#z"));
    }

    #[test]
    fn entity_start_hash_x_no_hex() {
        // &#x followed by non-hex — not valid
        assert!(!is_html_entity_start("&#x;"));
        assert!(!is_html_entity_start("&#xG"));
    }

    // ---- decode_html_entities ----

    #[test]
    fn decode_no_entities() {
        assert_eq!(decode_html_entities("hello world"), "hello world");
    }

    #[test]
    fn decode_named_entities() {
        assert_eq!(decode_html_entities("&lt;"), "<");
        assert_eq!(decode_html_entities("&gt;"), ">");
        assert_eq!(decode_html_entities("&amp;"), "&");
        assert_eq!(decode_html_entities("&quot;"), "\"");
        assert_eq!(decode_html_entities("&apos;"), "'");
    }

    #[test]
    fn decode_hex_entity() {
        assert_eq!(decode_html_entities("&#x3C;"), "<");
        assert_eq!(decode_html_entities("&#X3C;"), "<");
        assert_eq!(decode_html_entities("&#x3E;"), ">");
    }

    #[test]
    fn decode_decimal_entity() {
        assert_eq!(decode_html_entities("&#60;"), "<");
        assert_eq!(decode_html_entities("&#62;"), ">");
    }

    #[test]
    fn decode_mixed_text_and_entities() {
        assert_eq!(
            decode_html_entities("a &lt; b &amp;&amp; c &gt; d"),
            "a < b && c > d"
        );
    }

    #[test]
    fn decode_invalid_entity_preserved() {
        // Unknown named entity — kept as-is
        assert_eq!(decode_html_entities("&notarealentity;"), "&notarealentity;");
    }

    #[test]
    fn decode_bare_ampersand() {
        // & not followed by entity syntax — kept as-is
        assert_eq!(decode_html_entities("a & b"), "a & b");
    }

    #[test]
    fn decode_entity_without_semicolon() {
        // &lt without ; — not treated as entity
        assert_eq!(decode_html_entities("&lt "), "&lt ");
    }

    #[test]
    fn decode_empty_string() {
        assert_eq!(decode_html_entities(""), "");
    }

    #[test]
    fn decode_unicode_hex_entity() {
        // Unicode emoji via hex entity
        assert_eq!(decode_html_entities("&#x1F600;"), "😀");
    }

    #[test]
    fn decode_unicode_decimal_entity() {
        assert_eq!(decode_html_entities("&#128512;"), "😀");
    }

    #[test]
    fn decode_consecutive_entities() {
        assert_eq!(decode_html_entities("&lt;&gt;"), "<>");
    }

    // ---- decode_entity (tested through decode_html_entities) ----

    #[test]
    fn decode_entity_no_ampersand() {
        assert_eq!(decode_entity("lt;"), None);
    }

    #[test]
    fn decode_entity_no_semicolon() {
        assert_eq!(decode_entity("&lt"), None);
    }

    #[test]
    fn decode_entity_hex_case_insensitive() {
        assert_eq!(decode_entity("&#x3c;"), Some('<'));
        assert_eq!(decode_entity("&#X3C;"), Some('<'));
    }

    #[test]
    fn decode_entity_invalid_hex() {
        assert_eq!(decode_entity("&#xZZZ;"), None);
    }

    #[test]
    fn decode_entity_invalid_decimal() {
        assert_eq!(decode_entity("&#abc;"), None);
    }

    #[test]
    fn decode_entity_invalid_unicode_codepoint() {
        // U+D800 is a surrogate, not a valid char
        assert_eq!(decode_entity("&#xD800;"), None);
    }

    #[test]
    fn decode_entity_xml_entities_coverage() {
        // Spot-check some less common XML entities
        assert_eq!(decode_entity("&copy;"), Some('©'));
        assert_eq!(decode_entity("&reg;"), Some('®'));
        assert_eq!(decode_entity("&nbsp;"), Some('\u{00A0}'));
    }
}
