//! String escaping and HTML entity decoding utilities.
//!
//! Pure functions with no dependency on printer state. Used by the printer
//! to safely embed content inside JavaScript template literals, HTML attributes,
//! and quoted strings.

use cow_utils::CowUtils;

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
pub fn escape_double_quotes(s: &str) -> String {
    s.cow_replace('"', "\\\"").into_owned()
}

/// Escape single quotes for embedding inside a `'...'` string.
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
    let chars: Vec<char> = s.chars().collect();
    let mut i = 0;

    while i < chars.len() {
        if chars[i] == '&' {
            // Check if this is part of a valid HTML entity
            let remaining: String = chars[i..].iter().collect();
            if is_html_entity_start(&remaining) {
                // Keep the & as-is (part of an entity)
                result.push('&');
            } else {
                // Escape the &
                result.push_str("&amp;");
            }
        } else {
            result.push(chars[i]);
        }
        i += 1;
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

    // Named entities
    match inner {
        "lt" => Some('<'),
        "gt" => Some('>'),
        "amp" => Some('&'),
        "quot" => Some('"'),
        "apos" => Some('\''),
        "nbsp" => Some('\u{00A0}'),
        _ => None,
    }
}

/// Check if a string starting with `&` is a valid HTML entity start.
///
/// Used by [`escape_ampersands`] to distinguish standalone `&` characters
/// (which should be escaped to `&amp;`) from `&` that begins a real entity
/// (which should be preserved).
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

    // Named entity: & followed by alphanumeric, eventually ending with ;
    // Check if the next char is alphanumeric (common named entities like &quot;, &amp;, etc.)
    rest.chars()
        .next()
        .is_some_and(|c| c.is_ascii_alphanumeric())
}
