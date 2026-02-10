//! Sourcemap builder for Astro codegen.
//!
//! Maps generated JavaScript positions back to original `.astro` source positions.
//! This is used during Phase 1 of code generation (before TypeScript stripping).
//!
//! The design is adapted from `oxc_codegen`'s `SourcemapBuilder`, but simplified
//! for the Astro codegen's manual printing approach (as opposed to the `Gen` trait).

use std::path::Path;

use oxc_span::Span;

/// Sourcemap builder for the Astro codegen pass.
///
/// Tracks the mapping between positions in the generated JavaScript output
/// and positions in the original `.astro` source file.
pub struct AstroSourcemapBuilder {
    /// The source id assigned by the inner sourcemap builder.
    source_id: u32,
    /// The original `.astro` source text (used for byte-offset → line/column conversion).
    original_source: &'static str,
    /// Line offset table for the original source: `line_starts[i]` is the byte offset
    /// of the first character on line `i` (0-indexed).
    line_starts: Vec<u32>,
    /// The inner `oxc_sourcemap::SourceMapBuilder` that accumulates tokens.
    inner: oxc_sourcemap::SourceMapBuilder,

    // --- Generated position tracking ---
    /// Last byte length of the output buffer when we updated generated position.
    last_generated_update: usize,
    /// Current generated line (0-indexed).
    generated_line: u32,
    /// Current generated column (0-indexed, in UTF-16 code units for spec compliance).
    generated_column: u32,
    /// Last original position we emitted a mapping for (used to deduplicate).
    last_position: Option<u32>,
}

impl AstroSourcemapBuilder {
    /// Create a new sourcemap builder.
    ///
    /// `source_path` is the filename used in the sourcemap's `sources` array.
    /// `source_text` is the original `.astro` source text.
    ///
    /// # Safety / Lifetime
    /// The `source_text` reference is transmuted to `'static` to avoid lifetime
    /// infection throughout `AstroCodegen`. The caller must ensure the source text
    /// outlives this builder (which it does — both live for the duration of `build()`).
    pub fn new(source_path: &Path, source_text: &str) -> Self {
        let mut inner = oxc_sourcemap::SourceMapBuilder::default();
        // SAFETY: source_text is borrowed from the allocator which outlives the builder.
        let static_source: &'static str = unsafe { std::mem::transmute(source_text) };
        let source_id =
            inner.set_source_and_content(source_path.to_string_lossy().as_ref(), static_source);
        let line_starts = Self::compute_line_starts(source_text);
        Self {
            source_id,
            original_source: static_source,
            line_starts,
            inner,
            last_generated_update: 0,
            generated_line: 0,
            generated_column: 0,
            last_position: None,
        }
    }

    /// Consume the builder and produce the final `SourceMap`.
    pub fn into_sourcemap(self) -> oxc_sourcemap::SourceMap {
        self.inner.into_sourcemap()
    }

    /// Add a source mapping from the current generated position to the given
    /// original byte offset in the `.astro` source.
    ///
    /// `output` is the current contents of the generated code buffer (as bytes).
    /// `original_position` is a byte offset into the original `.astro` source text.
    pub fn add_source_mapping(&mut self, output: &[u8], original_position: u32) {
        self.add_source_mapping_impl(output, original_position, None);
    }

    /// Add a source mapping with an optional name.
    #[expect(dead_code)]
    pub fn add_source_mapping_with_name(
        &mut self,
        output: &[u8],
        original_position: u32,
        name: &str,
    ) {
        self.add_source_mapping_impl(output, original_position, Some(name));
    }

    /// Add a source mapping, bypassing the consecutive-position dedup check.
    ///
    /// Use this when multiple generated lines should map to the same original
    /// position (e.g. a multi-line expression that was expanded by codegen but
    /// originates from a single source span).
    pub fn add_source_mapping_force(&mut self, output: &[u8], original_position: u32) {
        // Temporarily clear last_position so the dedup check passes.
        self.last_position = None;
        self.add_source_mapping_impl(output, original_position, None);
    }

    fn add_source_mapping_impl(
        &mut self,
        output: &[u8],
        original_position: u32,
        name: Option<&str>,
    ) {
        // Deduplicate consecutive mappings to the same original position
        if self.last_position == Some(original_position) {
            return;
        }

        // Clamp position to source length
        let original_position =
            original_position.min(self.original_source.len().try_into().unwrap_or(u32::MAX));

        let (original_line, original_column) = self.byte_offset_to_line_column(original_position);
        self.update_generated_line_and_column(output);

        let name_id = name.map(|n| self.inner.add_name(n));

        self.inner.add_token(
            self.generated_line,
            self.generated_column,
            original_line,
            original_column,
            Some(self.source_id),
            name_id,
        );

        self.last_position = Some(original_position);
    }

    /// Add a mapping for a `Span`, using the span's start position.
    pub fn add_source_mapping_for_span(&mut self, output: &[u8], span: Span) {
        if !span.is_empty() {
            self.add_source_mapping(output, span.start);
        }
    }

    /// Convert a byte offset in the original source to (line, column), both 0-indexed.
    /// Column is counted in UTF-16 code units (per the sourcemap spec).
    #[expect(clippy::cast_possible_truncation)]
    fn byte_offset_to_line_column(&self, byte_offset: u32) -> (u32, u32) {
        // Binary search for the line
        let line = match self.line_starts.binary_search(&byte_offset) {
            Ok(exact) => exact,
            Err(insert_pos) => insert_pos.saturating_sub(1),
        };

        let line_start = self.line_starts[line];
        let byte_col = byte_offset - line_start;

        // Check if the segment is pure ASCII for fast path
        let line_end = byte_offset as usize;
        let line_start_usize = line_start as usize;
        let segment = &self.original_source.as_bytes()
            [line_start_usize..line_end.min(self.original_source.len())];

        let column = if segment.iter().all(u8::is_ascii) {
            byte_col
        } else {
            // Slow path: count UTF-16 code units
            let segment_str =
                &self.original_source[line_start_usize..line_end.min(self.original_source.len())];
            segment_str.encode_utf16().count() as u32
        };

        (line as u32, column)
    }

    /// Update `generated_line` and `generated_column` by scanning new bytes in `output`
    /// since the last update.
    #[expect(clippy::cast_possible_truncation)]
    fn update_generated_line_and_column(&mut self, output: &[u8]) {
        let start = self.last_generated_update;
        if start >= output.len() {
            self.last_generated_update = output.len();
            return;
        }

        let new_bytes = &output[start..];

        // Find the last newline in the new bytes
        let mut last_newline_pos = None;
        let mut newline_count: u32 = 0;

        let mut i = 0;
        while i < new_bytes.len() {
            let b = new_bytes[i];
            if b == b'\n' {
                newline_count += 1;
                last_newline_pos = Some(i);
            } else if b == b'\r' {
                newline_count += 1;
                // Handle \r\n as a single newline; advance past the \n
                if new_bytes.get(i + 1) == Some(&b'\n') {
                    i += 1;
                    last_newline_pos = Some(i);
                } else {
                    last_newline_pos = Some(i);
                }
            }
            i += 1;
        }

        if let Some(last_nl) = last_newline_pos {
            self.generated_line += newline_count;
            // Column is the number of bytes/chars after the last newline
            let after_last_newline = &new_bytes[last_nl + 1..];
            if after_last_newline.iter().all(u8::is_ascii) {
                self.generated_column = after_last_newline.len() as u32;
            } else {
                let s = std::str::from_utf8(after_last_newline).unwrap_or("");
                self.generated_column = s.encode_utf16().count() as u32;
            }
        } else {
            // No newlines — just advance the column
            if new_bytes.iter().all(u8::is_ascii) {
                self.generated_column += new_bytes.len() as u32;
            } else {
                let s = std::str::from_utf8(new_bytes).unwrap_or("");
                self.generated_column += s.encode_utf16().count() as u32;
            }
        }

        self.last_generated_update = output.len();
    }

    /// Compute line start byte offsets for the source text.
    #[expect(clippy::cast_possible_truncation)]
    fn compute_line_starts(source: &str) -> Vec<u32> {
        let mut starts = vec![0u32];
        for (i, b) in source.bytes().enumerate() {
            if b == b'\n' {
                starts.push((i + 1) as u32);
            } else if b == b'\r' {
                if source.as_bytes().get(i + 1) == Some(&b'\n') {
                    // \r\n — the \n will push the line start
                    continue;
                }
                starts.push((i + 1) as u32);
            }
        }
        starts
    }
}

/// Compose two sourcemaps: `final_code → intermediate_code` (phase2) and
/// `intermediate_code → original_source` (phase1), producing `final_code → original_source`.
///
/// This is used to chain the TypeScript stripping pass's sourcemap with the
/// Astro codegen's sourcemap.
pub fn remap_sourcemap(
    phase2_map: &oxc_sourcemap::SourceMap,
    phase1_map: &oxc_sourcemap::SourceMap,
    source_path: &str,
    source_content: &str,
) -> oxc_sourcemap::SourceMap {
    let lookup = phase1_map.generate_lookup_table();

    let mut builder = oxc_sourcemap::SourceMapBuilder::default();
    let source_id = builder.set_source_and_content(source_path, source_content);

    for token in phase2_map.get_tokens() {
        // token maps: (dst_line, dst_col) in final code → (src_line, src_col) in intermediate code
        // We look up (src_line, src_col) in phase1_map to find the original .astro position
        let intermediate_line = token.get_src_line();
        let intermediate_col = token.get_src_col();

        if let Some(original_token) =
            phase1_map.lookup_token(&lookup, intermediate_line, intermediate_col)
        {
            // Found a mapping in phase1: intermediate → original .astro source
            let name_id = original_token
                .get_name_id()
                .and_then(|id| phase1_map.get_name(id))
                .map(|name| builder.add_name(name));

            builder.add_token(
                token.get_dst_line(),
                token.get_dst_col(),
                original_token.get_src_line(),
                original_token.get_src_col(),
                Some(source_id),
                name_id,
            );
        }
        // If no mapping found in phase1, we skip this token (generated code with no original source)
    }

    builder.into_sourcemap()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_compute_line_starts() {
        let source = "line1\nline2\nline3";
        let starts = AstroSourcemapBuilder::compute_line_starts(source);
        assert_eq!(starts, vec![0, 6, 12]);
    }

    #[test]
    fn test_compute_line_starts_crlf() {
        let source = "line1\r\nline2\r\nline3";
        let starts = AstroSourcemapBuilder::compute_line_starts(source);
        assert_eq!(starts, vec![0, 7, 14]);
    }

    #[test]
    fn test_byte_offset_to_line_column() {
        let source = "abc\ndef\nghi";
        let builder = AstroSourcemapBuilder::new(Path::new("test.astro"), source);

        // 'a' at offset 0 → line 0, col 0
        assert_eq!(builder.byte_offset_to_line_column(0), (0, 0));
        // 'c' at offset 2 → line 0, col 2
        assert_eq!(builder.byte_offset_to_line_column(2), (0, 2));
        // 'd' at offset 4 → line 1, col 0
        assert_eq!(builder.byte_offset_to_line_column(4), (1, 0));
        // 'g' at offset 8 → line 2, col 0
        assert_eq!(builder.byte_offset_to_line_column(8), (2, 0));
        // 'i' at offset 10 → line 2, col 2
        assert_eq!(builder.byte_offset_to_line_column(10), (2, 2));
    }

    #[test]
    fn test_basic_mapping() {
        let source = "hello\nworld";
        let mut builder = AstroSourcemapBuilder::new(Path::new("test.astro"), source);

        let output = b"const x = 'hello';\n";
        builder.add_source_mapping(output, 0);

        let map = builder.into_sourcemap();
        let tokens: Vec<_> = map.get_tokens().collect();
        assert_eq!(tokens.len(), 1);
        assert_eq!(tokens[0].get_src_line(), 0);
        assert_eq!(tokens[0].get_src_col(), 0);
    }
}
