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
