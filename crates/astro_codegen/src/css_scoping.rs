//! CSS scoping for Astro components.
//!
//! Transforms CSS selectors within `<style>` blocks to include scope identifiers,
//! matching the behavior of the Go compiler's CSS scoping (using a vendored esbuild).
//!
//! We use lightningcss for CSS parsing of the overall stylesheet structure,
//! while performing selector scoping via the visitor pattern.
//!
//! CSS modules mode is enabled at parse time so that `:global()` is parsed as a
//! first-class `PseudoClass::Global { selector }` node (a fully parsed `Selector`),
//! rather than a raw `CustomFunction` with opaque `TokenList`. CSS modules is then
//! disabled before printing so no class renaming occurs.

use std::convert::Infallible;

use lightningcss::css_modules;
use lightningcss::printer::PrinterOptions;
use lightningcss::selector::{Combinator, Component, PseudoClass, Selector, SelectorList};
use lightningcss::stylesheet::{ParserFlags, ParserOptions, StyleSheet};
use lightningcss::values::ident::Ident;
use lightningcss::values::string::CowArcStr;
use lightningcss::visit_types;
use lightningcss::visitor::{Visit, VisitTypes, Visitor};
use smallvec::smallvec;

use crate::ScopedStyleStrategy;

/// Scope CSS selectors.
///
/// Parses the CSS, identifies style rules, transforms their selectors to include
/// the scope identifier, and returns the CSS.
///
/// If parsing fails, returns the original CSS unchanged.
pub fn scope_css(css: &str, scope: &str, strategy: ScopedStyleStrategy) -> String {
    // Parse with nesting support, error recovery, and CSS modules mode.
    // CSS modules mode makes `:global()` parse as `PseudoClass::Global { selector }`
    // instead of `PseudoClass::CustomFunction { name: "global", arguments: TokenList }`.
    let options = ParserOptions {
        flags: ParserFlags::NESTING,
        error_recovery: true,
        css_modules: Some(css_modules::Config {
            // Identity pattern: [local] means names are output unchanged (no hashing).
            pattern: css_modules::Pattern {
                segments: smallvec![css_modules::Segment::Local],
            },
            animation: false,
            grid: false,
            custom_idents: false,
            container: false,
            dashed_idents: false,
            pure: false,
        }),
        ..ParserOptions::default()
    };

    let mut stylesheet = match StyleSheet::parse(css, options) {
        Ok(ss) => ss,
        Err(_) => return css.to_string(),
    };

    // Visit all selectors and inject scope identifiers
    let mut visitor = ScopeVisitor { scope, strategy };
    let _ = stylesheet.visit(&mut visitor);

    // Print — no minification, that's handled elsewhere in the pipeline.
    // CSS modules is active but with an identity pattern ([local]), so no renaming occurs.
    // Any remaining `PseudoClass::Global { selector }` nodes will have their `:global()`
    // wrapper stripped by the printer (it serializes the inner selector directly).
    let result = stylesheet
        .to_css(PrinterOptions::default())
        .unwrap_or_else(|_| lightningcss::stylesheet::ToCssResult {
            code: css.to_string(),
            exports: None,
            references: None,
            dependencies: None,
        });

    result.code
}

// ---------------------------------------------------------------------------
// Scope visitor
// ---------------------------------------------------------------------------

struct ScopeVisitor<'a> {
    scope: &'a str,
    strategy: ScopedStyleStrategy,
}

impl<'i> Visitor<'i> for ScopeVisitor<'_> {
    type Error = Infallible;

    fn visit_types(&self) -> VisitTypes {
        visit_types!(SELECTORS)
    }

    fn visit_selector_list(&mut self, selectors: &mut SelectorList<'i>) -> Result<(), Self::Error> {
        let new_selectors: Vec<Selector<'i>> = selectors
            .0
            .iter()
            .flat_map(|selector| self.scope_selector(selector))
            .collect();

        selectors.0 = new_selectors.into();
        Ok(())
    }
}

impl ScopeVisitor<'_> {
    /// Create the scope component based on the strategy.
    fn scope_component<'i>(&self) -> Component<'i> {
        match self.strategy {
            ScopedStyleStrategy::Where => {
                // :where(.astro-XXXX)
                let class_component =
                    Component::Class(Ident(format!("astro-{}", self.scope).into()));
                let inner_selector: Selector<'i> = vec![class_component].into();
                Component::Where(Box::new([inner_selector]))
            }
            ScopedStyleStrategy::Class => {
                // .astro-XXXX
                Component::Class(Ident(format!("astro-{}", self.scope).into()))
            }
            ScopedStyleStrategy::Attribute => {
                // [data-astro-cid-XXXX]
                let attr_name: CowArcStr<'i> = format!("data-astro-cid-{}", self.scope).into();
                Component::AttributeInNoNamespaceExists {
                    local_name: Ident(attr_name.clone()),
                    local_name_lower: Ident(attr_name),
                }
            }
        }
    }

    /// Scope a single selector, potentially returning multiple selectors.
    fn scope_selector<'i>(&self, selector: &Selector<'i>) -> Vec<Selector<'i>> {
        let compounds = self.split_into_compounds(selector);

        // Merge pseudo-element "compounds" back into their preceding compound.
        // Internally, `::before` is stored as [Combinator::PseudoElement, PseudoElement(Before)]
        // which splits into a separate compound. We merge it back so scoping treats
        // `h3::before` as one unit.
        let merged = self.merge_pseudo_element_compounds(&compounds);

        let mut result_components: Vec<Component<'i>> = Vec::new();

        for (i, (combinator, compound)) in merged.iter().enumerate() {
            if i > 0
                && let Some(comb) = combinator
            {
                result_components.push(Component::Combinator(*comb));
            }

            let scoped = self.scope_compound(compound);
            result_components.extend(scoped);
        }

        if result_components.is_empty() {
            return vec![];
        }

        vec![result_components.into()]
    }

    /// Merge pseudo-element compounds back into the preceding compound.
    /// `Combinator::PseudoElement` is an internal marker, not a real CSS combinator.
    fn merge_pseudo_element_compounds<'i>(
        &self,
        compounds: &[(Option<Combinator>, Vec<Component<'i>>)],
    ) -> Vec<(Option<Combinator>, Vec<Component<'i>>)> {
        let mut result: Vec<(Option<Combinator>, Vec<Component<'i>>)> = Vec::new();

        for (combinator, compound) in compounds {
            if matches!(combinator, Some(Combinator::PseudoElement)) {
                // Merge into the previous compound
                if let Some(last) = result.last_mut() {
                    last.1.extend(compound.iter().cloned());
                } else {
                    // No previous compound — just push as-is
                    result.push((*combinator, compound.clone()));
                }
            } else {
                result.push((*combinator, compound.clone()));
            }
        }

        result
    }

    /// Split a selector into (combinator, compound_components) pairs in parse order.
    /// Components within each compound are in their original internal order
    /// (which is the order SelectorBuilder stores them — parse order within compound).
    fn split_into_compounds<'i>(
        &self,
        selector: &Selector<'i>,
    ) -> Vec<(Option<Combinator>, Vec<Component<'i>>)> {
        // Use the same split approach as the lightningcss serializer:
        // split the match-order slice by combinators, then reverse the compound order.
        let raw_slice = selector.iter_raw_match_order().as_slice();
        let mut combinators = selector
            .iter_raw_match_order()
            .rev()
            .filter_map(|x| x.as_combinator());

        // Split by combinators gives us compound slices in match order.
        // Reverse to get parse order (left-to-right).
        let compound_slices: Vec<&[Component<'i>]> =
            raw_slice.split(|x| x.is_combinator()).rev().collect();

        let mut result = Vec::with_capacity(compound_slices.len());
        for (i, compound) in compound_slices.iter().enumerate() {
            let combinator = if i == 0 { None } else { combinators.next() };
            let components: Vec<Component<'i>> = compound.to_vec();
            result.push((combinator, components));
        }

        result
    }

    /// Scope a single compound selector, returning the scoped components.
    fn scope_compound<'i>(&self, compound: &[Component<'i>]) -> Vec<Component<'i>> {
        if compound.is_empty() {
            return vec![];
        }

        // Check for nesting selector `&`
        if self.has_nesting(compound) {
            return self.scope_nesting_compound(compound);
        }

        // Check if the entire compound is or starts with :global()
        if self.is_global_compound(compound) {
            return self.process_global_compound(compound);
        }

        // Check for :root — never scoped
        if compound.len() == 1 && matches!(&compound[0], Component::Root) {
            return compound.to_vec();
        }

        // Check for body/html type selectors — never scoped at the compound level
        if self.is_body_or_html(compound) {
            return compound.to_vec();
        }

        // Normal compound: inject scope
        self.inject_scope_into_compound(compound)
    }

    /// Check if a compound contains a nesting selector `&`.
    fn has_nesting(&self, compound: &[Component<'_>]) -> bool {
        compound.iter().any(|c| matches!(c, Component::Nesting))
    }

    /// Check if a compound selector contains :global().
    fn is_global_compound(&self, compound: &[Component<'_>]) -> bool {
        compound.iter().any(|c| self.is_global_pseudo(c))
    }

    /// Check if a component is the :global() pseudo-class.
    fn is_global_pseudo(&self, component: &Component<'_>) -> bool {
        matches!(
            component,
            Component::NonTSPseudoClass(PseudoClass::Global { .. })
        )
    }

    /// Check if the compound has body or html as the type selector.
    fn is_body_or_html(&self, compound: &[Component<'_>]) -> bool {
        compound.iter().any(|c| match c {
            Component::LocalName(local) => {
                let name = local.name.0.as_ref();
                name == "body" || name == "html"
            }
            _ => false,
        })
    }

    /// Extract components from a `PseudoClass::Global { selector }` in parse order.
    ///
    /// The internal `Vec` stores compounds right-to-left (match order) with
    /// intra-compound components in parse order. We must reverse the *compound*
    /// order while preserving each compound's internal order — the same approach
    /// lightningcss's own `serialize_selector` uses.
    ///
    /// A naive `.rev()` would also reverse intra-compound order, placing
    /// pseudo-classes before type selectors (`:not(.x)html` instead of
    /// `html:not(.x)`).
    fn extract_global_components<'i>(selector: &Selector<'i>) -> Vec<Component<'i>> {
        let raw = selector.iter_raw_match_order().as_slice();

        // Collect combinators in parse order (reversed from match order).
        let combinators: Vec<Combinator> = selector
            .iter_raw_match_order()
            .rev()
            .filter_map(|c| c.as_combinator())
            .collect();

        // Split the raw slice by combinator entries.  Each resulting slice is
        // one compound, and the overall order of slices is match order
        // (rightmost compound first).  Reversing gives parse order.
        let compound_slices: Vec<&[Component<'i>]> =
            raw.split(|c| c.is_combinator()).rev().collect();

        let mut result = Vec::new();
        for (i, compound) in compound_slices.iter().enumerate() {
            if i > 0
                && let Some(comb) = combinators.get(i - 1)
            {
                result.push(Component::Combinator(*comb));
            }
            // Clone each component in its original (parse) order within the compound.
            result.extend(compound.iter().cloned());
        }

        result
    }

    /// Process a compound that contains :global() — strip the wrapper and return
    /// inner content unscoped, while scoping non-global parts.
    fn process_global_compound<'i>(&self, compound: &[Component<'i>]) -> Vec<Component<'i>> {
        let mut result = Vec::new();
        let mut has_non_global = false;

        for component in compound {
            if let Component::NonTSPseudoClass(PseudoClass::Global { selector }) = component {
                let inner_components = Self::extract_global_components(selector);
                result.extend(inner_components);
                continue;
            }
            has_non_global = true;
            result.push(component.clone());
        }

        // After expanding :global(), check if the result contains body/html — if so, no scoping
        if self.is_body_or_html(&result) {
            return result;
        }

        // If there were non-global parts mixed in (e.g., `.class:global(.bar)`),
        // we need to scope the non-global parts.
        if has_non_global {
            return self.inject_scope_into_compound(&result);
        }

        result
    }

    /// Scope a compound that contains a nesting selector `&`.
    fn scope_nesting_compound<'i>(&self, compound: &[Component<'i>]) -> Vec<Component<'i>> {
        // If the compound is just `&` alone, return as-is
        if compound.len() == 1 && matches!(&compound[0], Component::Nesting) {
            return compound.to_vec();
        }

        // Check if compound has a pseudo-element but no "real" selectors
        let has_pseudo_element = compound.iter().any(|c| is_pseudo_element(c));
        let has_real_selector = compound.iter().any(|c| is_real_selector(c));

        if has_pseudo_element && !has_real_selector {
            // e.g., `&::after` — inject scope before the pseudo-element
            let mut result = Vec::new();
            let mut scope_inserted = false;
            for c in compound {
                if is_pseudo_element(c) && !scope_inserted {
                    result.push(self.scope_component());
                    scope_inserted = true;
                }
                result.push(c.clone());
            }
            return result;
        }

        // Otherwise, `&.dark`, `&:hover`, etc. — return as-is
        compound.to_vec()
    }

    /// Inject the scope component into a compound selector.
    fn inject_scope_into_compound<'i>(&self, compound: &[Component<'i>]) -> Vec<Component<'i>> {
        let scope_component = self.scope_component();
        let mut result = Vec::new();
        let mut scoped = false;

        // Check if compound consists entirely of pseudo-class/pseudo-element
        let only_pseudo = compound
            .iter()
            .all(|c| is_pseudo_class(c) || is_pseudo_element(c));

        for (i, component) in compound.iter().enumerate() {
            match component {
                Component::ExplicitUniversalType => {
                    // `*` — replace with scope
                    result.push(scope_component.clone());
                    scoped = true;
                }
                Component::LocalName(_) => {
                    // Type selector: emit it, then scope immediately after
                    result.push(component.clone());
                    if !scoped {
                        result.push(scope_component.clone());
                        scoped = true;
                    }
                }
                Component::Class(_) | Component::ID(_) => {
                    result.push(component.clone());
                    if !scoped {
                        result.push(scope_component.clone());
                        scoped = true;
                    }
                }
                Component::AttributeInNoNamespaceExists { .. }
                | Component::AttributeInNoNamespace { .. }
                | Component::AttributeOther(_) => {
                    if !scoped {
                        result.push(scope_component.clone());
                        scoped = true;
                    }
                    result.push(component.clone());
                }
                Component::PseudoElement(_) => {
                    if !scoped {
                        result.push(scope_component.clone());
                        scoped = true;
                    }
                    result.push(component.clone());
                }
                Component::NonTSPseudoClass(pseudo) => {
                    // Check for :global() — shouldn't normally reach here since
                    // is_global_compound catches it earlier, but handle just in case.
                    if let PseudoClass::Global { selector } = pseudo {
                        let inner_components = Self::extract_global_components(selector);
                        result.extend(inner_components);
                        scoped = true;
                        continue;
                    }

                    // Other pseudo-classes
                    if only_pseudo && i == 0 && !scoped {
                        result.push(scope_component.clone());
                        scoped = true;
                    }
                    result.push(component.clone());
                }
                Component::Root => {
                    result.push(component.clone());
                    scoped = true;
                }
                _ => {
                    if only_pseudo && i == 0 && !scoped {
                        result.push(scope_component.clone());
                        scoped = true;
                    }
                    result.push(component.clone());
                }
            }
        }

        if !scoped {
            result.push(scope_component);
        }

        result
    }
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

/// Check if a component is a pseudo-element.
fn is_pseudo_element(component: &Component<'_>) -> bool {
    matches!(component, Component::PseudoElement(_))
}

/// Check if a component is a "real" selector (type, class, id, attribute).
fn is_real_selector(component: &Component<'_>) -> bool {
    matches!(
        component,
        Component::LocalName(_)
            | Component::Class(_)
            | Component::ID(_)
            | Component::AttributeInNoNamespaceExists { .. }
            | Component::AttributeInNoNamespace { .. }
            | Component::AttributeOther(_)
    )
}

/// Check if a component is a pseudo-class.
fn is_pseudo_class(component: &Component<'_>) -> bool {
    matches!(
        component,
        Component::NonTSPseudoClass(_)
            | Component::Negation(_)
            | Component::Root
            | Component::Empty
            | Component::Scope
            | Component::Nth(_)
            | Component::NthOf(_)
            | Component::Is(_)
            | Component::Where(_)
            | Component::Has(_)
    )
}

// ---------------------------------------------------------------------------
// Public helpers for the codegen pipeline
// ---------------------------------------------------------------------------

/// Elements that should never receive a scope class in the HTML.
pub const NEVER_SCOPED_ELEMENTS: &[&str] = &[
    "Fragment", "base", "font", "frame", "frameset", "head", "link", "meta", "noframes",
    "noscript", "script", "style", "slot", "title",
];

/// Check if an element should receive a scope class.
pub fn should_scope_element(name: &str) -> bool {
    !NEVER_SCOPED_ELEMENTS.contains(&name)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn scope(source: &str) -> String {
        scope_css(source, "xxxxxx", ScopedStyleStrategy::Where)
    }

    #[allow(dead_code)]
    fn scope_class(source: &str) -> String {
        scope_css(source, "xxxxxx", ScopedStyleStrategy::Class)
    }

    #[allow(dead_code)]
    fn scope_attribute(source: &str) -> String {
        scope_css(source, "xxxxxx", ScopedStyleStrategy::Attribute)
    }

    // === Basic selectors ===
    //
    // Note: lightningcss pretty-prints CSS (spaces around combinators, newlines,
    // indentation, trailing newline). It also normalizes:
    // - `::before`/`::after` → `:before`/`:after`
    // - attribute values get quoted
    // - media queries modernized: `min-width: 640px` → `width >= 640px`
    // - colors shortened: `blue` → `#00f`, `black` → `#000`, etc.
    // - `rotate(0deg)` → `rotate(0)`
    // - SelectorBuilder may reorder components within compounds

    #[test]
    fn test_class() {
        assert_eq!(scope(".class{}"), ".class:where(.astro-xxxxxx) {\n}\n");
    }

    #[test]
    fn test_id() {
        assert_eq!(scope("#class{}"), "#class:where(.astro-xxxxxx) {\n}\n");
    }

    #[test]
    fn test_element() {
        assert_eq!(scope("h1{}"), "h1:where(.astro-xxxxxx) {\n}\n");
    }

    #[test]
    fn test_adjacent_sibling() {
        assert_eq!(
            scope(".class+.class{}"),
            ".class:where(.astro-xxxxxx) + .class:where(.astro-xxxxxx) {\n}\n"
        );
    }

    #[test]
    fn test_and_selector() {
        assert_eq!(
            scope(".class,.class{}"),
            ".class:where(.astro-xxxxxx), .class:where(.astro-xxxxxx) {\n}\n"
        );
    }

    #[test]
    fn test_children_universal() {
        assert_eq!(
            scope(".class *{}"),
            ".class:where(.astro-xxxxxx) :where(.astro-xxxxxx) {\n}\n"
        );
    }

    #[test]
    fn test_attr() {
        assert_eq!(
            scope("a[aria-current=page]{}"),
            "a:where(.astro-xxxxxx)[aria-current=\"page\"] {\n}\n"
        );
    }

    #[test]
    fn test_attr_universal_implied() {
        assert_eq!(
            scope("[aria-visible],[aria-hidden]{}"),
            ":where(.astro-xxxxxx)[aria-visible], :where(.astro-xxxxxx)[aria-hidden] {\n}\n"
        );
    }

    #[test]
    fn test_universal_pseudo_state() {
        assert_eq!(scope("*:hover{}"), ":where(.astro-xxxxxx):hover {\n}\n");
    }

    #[test]
    fn test_immediate_child_universal() {
        assert_eq!(
            scope(".class>*{}"),
            ".class:where(.astro-xxxxxx) > :where(.astro-xxxxxx) {\n}\n"
        );
    }

    #[test]
    fn test_element_pseudo_state() {
        assert_eq!(
            scope(".class button:focus{}"),
            ".class:where(.astro-xxxxxx) button:where(.astro-xxxxxx):focus {\n}\n"
        );
    }

    #[test]
    fn test_element_pseudo_element() {
        assert_eq!(
            scope(".class h3::before{}"),
            ".class:where(.astro-xxxxxx) h3:where(.astro-xxxxxx):before {\n}\n"
        );
    }

    #[test]
    fn test_element_pseudo_state_pseudo_element() {
        assert_eq!(
            scope("button:focus::before{}"),
            "button:where(.astro-xxxxxx):focus:before {\n}\n"
        );
    }

    #[test]
    fn test_media_query() {
        assert_eq!(
            scope("@media screen and (min-width:640px){.class{}}"),
            "@media screen and (width >= 640px) {\n  .class:where(.astro-xxxxxx) {\n  }\n}\n"
        );
    }

    #[test]
    fn test_global_children() {
        assert_eq!(
            scope(".class :global(ul li){}"),
            ".class:where(.astro-xxxxxx) ul li {\n}\n"
        );
    }

    #[test]
    fn test_global_universal() {
        assert_eq!(
            scope(".class :global(*){}"),
            ".class:where(.astro-xxxxxx) * {\n}\n"
        );
    }

    #[test]
    fn test_global_with_scoped_children() {
        assert_eq!(
            scope(":global(section) .class{}"),
            "section .class:where(.astro-xxxxxx) {\n}\n"
        );
    }

    #[test]
    fn test_subsequent_siblings_global() {
        assert_eq!(
            scope(".class~:global(a){}"),
            ".class:where(.astro-xxxxxx) ~ a {\n}\n"
        );
    }

    #[test]
    fn test_global_nested_parens() {
        assert_eq!(
            scope(".class :global(.nav:not(.is-active)){}"),
            ".class:where(.astro-xxxxxx) .nav:not(.is-active) {\n}\n"
        );
    }

    #[test]
    fn test_global_chaining_global() {
        assert_eq!(scope(":global(.foo):global(.bar){}"), ".foo.bar {\n}\n");
    }

    #[test]
    fn test_class_chained_global() {
        assert_eq!(
            scope(".class:global(.bar){}"),
            ".class:where(.astro-xxxxxx).bar {\n}\n"
        );
    }

    #[test]
    fn test_body() {
        assert_eq!(scope("body h1{}"), "body h1:where(.astro-xxxxxx) {\n}\n");
    }

    #[test]
    fn test_body_class() {
        assert_eq!(scope("body.theme-dark{}"), "body.theme-dark {\n}\n");
    }

    #[test]
    fn test_html_and_body() {
        assert_eq!(scope("html,body{}"), "html, body {\n}\n");
    }

    #[test]
    fn test_root() {
        assert_eq!(scope(":root{}"), ":root {\n}\n");
    }

    #[test]
    fn test_chained_not() {
        assert_eq!(
            scope(".class:not(.is-active):not(.is-disabled){}"),
            ".class:where(.astro-xxxxxx):not(.is-active):not(.is-disabled) {\n}\n"
        );
    }

    #[test]
    fn test_weird_chaining() {
        assert_eq!(
            scope(":hover.a:focus{}"),
            ":hover.a:where(.astro-xxxxxx):focus {\n}\n"
        );
    }

    #[test]
    fn test_more_weird_chaining() {
        assert_eq!(
            scope(":not(.is-disabled).a{}"),
            ":not(.is-disabled).a:where(.astro-xxxxxx) {\n}\n"
        );
    }

    #[test]
    fn test_keyframes() {
        assert_eq!(
            scope("@keyframes shuffle{from{transform:rotate(0deg);}to{transform:rotate(360deg);}}"),
            "@keyframes shuffle {\n  from {\n    transform: rotate(0);\n  }\n\n  to {\n    transform: rotate(360deg);\n  }\n}\n"
        );
    }

    #[test]
    fn test_variables() {
        assert_eq!(
            scope("body{--bg:red;background:var(--bg);color:black;}"),
            "body {\n  --bg: red;\n  background: var(--bg);\n  color: #000;\n}\n"
        );
    }

    #[test]
    fn test_calc() {
        assert_eq!(
            scope(":root{padding:calc(var(--space) * 2);}"),
            ":root {\n  padding: calc(var(--space) * 2);\n}\n"
        );
    }

    #[test]
    fn test_class_strategy() {
        assert_eq!(scope_class(".class{}"), ".class.astro-xxxxxx {\n}\n");
    }

    #[test]
    fn test_attribute_strategy() {
        assert_eq!(
            scope_attribute(".class{}"),
            ".class[data-astro-cid-xxxxxx] {\n}\n"
        );
    }

    #[test]
    fn test_nesting_combinator() {
        assert_eq!(
            scope("div{& span{color:blue}}"),
            "div:where(.astro-xxxxxx) {\n  & span:where(.astro-xxxxxx) {\n    color: #00f;\n  }\n}\n"
        );
    }

    #[test]
    fn test_nesting_modifier() {
        assert_eq!(
            scope(".header{background-color:white;&.dark{background-color:blue}}"),
            ".header:where(.astro-xxxxxx) {\n  background-color: #fff;\n\n  &.dark {\n    background-color: #00f;\n  }\n}\n"
        );
    }

    #[test]
    fn test_container() {
        assert_eq!(
            scope("@container (min-width: 200px) and (min-height: 200px){h1{font-size:30px}}"),
            "@container (width >= 200px) and (height >= 200px) {\n  h1:where(.astro-xxxxxx) {\n    font-size: 30px;\n  }\n}\n"
        );
    }

    #[test]
    fn test_layer() {
        assert_eq!(
            scope("@layer theme,layout,utilities;@layer special{.item{color:rebeccapurple}}"),
            "@layer theme, layout, utilities;\n\n@layer special {\n  .item:where(.astro-xxxxxx) {\n    color: #639;\n  }\n}\n"
        );
    }

    #[test]
    fn test_starting_style() {
        assert_eq!(
            scope("@starting-style{.class{}}"),
            "@starting-style {\n  .class:where(.astro-xxxxxx) {\n  }\n}\n"
        );
    }

    #[test]
    fn test_only_pseudo_element() {
        assert_eq!(
            scope(".class>::before{}"),
            ".class:where(.astro-xxxxxx) > :where(.astro-xxxxxx):before {\n}\n"
        );
    }

    #[test]
    fn test_only_pseudo_class_and_pseudo_element() {
        assert_eq!(
            scope(".class>:not(:first-child)::after{}"),
            ".class:where(.astro-xxxxxx) > :where(.astro-xxxxxx):not(:first-child):after {\n}\n"
        );
    }

    #[test]
    fn test_escaped_characters() {
        assert_eq!(
            scope(".class\\:class:focus{}"),
            ".class\\:class:where(.astro-xxxxxx):focus {\n}\n"
        );
    }

    #[test]
    fn test_nested_only_pseudo_element() {
        assert_eq!(
            scope(".class{& .other_class{&::after{}}}"),
            ".class:where(.astro-xxxxxx) {\n  & .other_class:where(.astro-xxxxxx) {\n    &:where(.astro-xxxxxx):after {\n    }\n  }\n}\n"
        );
    }

    // === Global with nested at-rules ===

    #[test]
    fn test_global_nested_media() {
        assert_eq!(
            scope(
                ":global(html) { @media (min-width: 640px) { color: blue } }html { background-color: lime }"
            ),
            "html {\n  @media (width >= 640px) {\n    color: #00f;\n  }\n}\n\nhtml {\n  background-color: #0f0;\n}\n"
        );
    }

    // Additional Go compiler tests

    #[test]
    fn test_global_nested_parens_chained() {
        assert_eq!(
            scope(":global(body:not(.is-light)).is-dark,:global(body:not(.is-dark)).is-light{}"),
            "body:not(.is-light).is-dark, body:not(.is-dark).is-light {\n}\n"
        );
    }

    #[test]
    fn test_global_compound_with_not() {
        // Regression: `:global(html:not(.theme-dark))` must keep the type selector
        // `html` before `:not(.theme-dark)`, not produce invalid `:not(.theme-dark)html`.
        assert_eq!(
            scope(
                ":global(.theme-dark) .icon.dark, :global(html:not(.theme-dark)) .icon.light, button[aria-pressed='false'] .icon.light { color: var(--accent-text-over); }"
            ),
            ".theme-dark .icon:where(.astro-xxxxxx).dark, html:not(.theme-dark) .icon:where(.astro-xxxxxx).light, button:where(.astro-xxxxxx)[aria-pressed=\"false\"] .icon:where(.astro-xxxxxx).light {\n  color: var(--accent-text-over);\n}\n"
        );
    }

    #[test]
    fn test_keyframes_with_selectors() {
        assert_eq!(
            scope(
                "@keyframes shuffle{0%{transform:rotate(0deg);color:blue}100%{transform:rotate(360deg)}} h1{} h2{}"
            ),
            "@keyframes shuffle {\n  0% {\n    transform: rotate(0);\n    color: #00f;\n  }\n\n  100% {\n    transform: rotate(360deg);\n  }\n}\n\nh1:where(.astro-xxxxxx) {\n}\n\nh2:where(.astro-xxxxxx) {\n}\n"
        );
    }
}
