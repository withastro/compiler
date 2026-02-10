//! Slot analysis and printing.
//!
//! This module handles two concerns:
//!
//! 1. **Slot analysis** — pure functions that inspect JSX children and expressions
//!    to determine which slots they belong to (`SlotValue`, `ExpressionSlotInfo`,
//!    `extract_slots_from_expression`, etc.).
//!
//! 2. **Slot printing** — `impl AstroCodegen` methods that emit the slot objects
//!    passed to `$$renderComponent` (`print_component_slots`,
//!    `print_conditional_slot_*`, etc.).

use oxc_ast::ast::*;
use oxc_span::GetSpan;

use super::AstroCodegen;
use super::escape::escape_double_quotes;
use super::runtime;
use super::{expr_to_string, gen_to_string};

/// Represents a slot attribute value — either a static string or a dynamic expression.
#[derive(Debug, Clone)]
pub(super) enum SlotValue {
    /// Static slot name like `slot="header"`.
    Static(String),
    /// Dynamic slot name like `slot={name}` — stores the expression as a string and the attribute span.
    Dynamic(String, oxc_span::Span),
}

/// Extract the static slot name from a JSX element's attributes.
///
/// Returns `Some("name")` if the element has `slot="name"`, otherwise `None`.
/// Does **not** handle dynamic `slot={expr}` — use [`get_slot_attribute_value`] for that.
pub(super) fn get_slot_attribute<'a>(attrs: &'a [JSXAttributeItem<'a>]) -> Option<&'a str> {
    for attr in attrs {
        if let JSXAttributeItem::Attribute(attr) = attr
            && let JSXAttributeName::Identifier(ident) = &attr.name
            && ident.name.as_str() == "slot"
            && let Some(JSXAttributeValue::StringLiteral(lit)) = &attr.value
        {
            return Some(lit.value.as_str());
        }
    }
    None
}

/// Extract the slot attribute value (static or dynamic) from a JSX element's attributes.
pub(super) fn get_slot_attribute_value(attrs: &[JSXAttributeItem<'_>]) -> Option<SlotValue> {
    for attr in attrs {
        if let JSXAttributeItem::Attribute(attr) = attr
            && let JSXAttributeName::Identifier(ident) = &attr.name
            && ident.name.as_str() == "slot"
        {
            match &attr.value {
                Some(JSXAttributeValue::StringLiteral(lit)) => {
                    return Some(SlotValue::Static(lit.value.to_string()));
                }
                Some(JSXAttributeValue::ExpressionContainer(expr)) => {
                    if let Some(e) = expr.expression.as_expression() {
                        return Some(SlotValue::Dynamic(expr_to_string(e), attr.span));
                    }
                }
                _ => {}
            }
        }
    }
    None
}

/// Information about slots found within an expression container.
#[derive(Debug)]
pub(super) enum ExpressionSlotInfo<'a> {
    /// No slotted elements found — treat as default slot.
    None,
    /// Single slot found — use that slot name for the entire expression.
    Single(&'a str),
    /// Multiple different slots found — requires `$$mergeSlots`.
    Multiple,
}

/// Extract slot information from a JSX expression.
///
/// Recursively searches for JSX elements with `slot` attributes.
pub(super) fn extract_slots_from_expression<'a>(
    expr: &'a JSXExpression<'a>,
) -> ExpressionSlotInfo<'a> {
    let mut slots = Vec::new();
    collect_slots_from_expression(expr, &mut slots);

    match slots.len() {
        0 => ExpressionSlotInfo::None,
        1 => ExpressionSlotInfo::Single(slots[0]),
        _ => {
            // Check if all slots are the same
            if slots.iter().all(|s| *s == slots[0]) {
                ExpressionSlotInfo::Single(slots[0])
            } else {
                ExpressionSlotInfo::Multiple
            }
        }
    }
}

/// Recursively collect slot names from a JSX expression.
fn collect_slots_from_expression<'a>(expr: &'a JSXExpression<'a>, slots: &mut Vec<&'a str>) {
    match expr {
        JSXExpression::JSXElement(el) => {
            if let Some(slot_name) = get_slot_attribute(&el.opening_element.attributes) {
                slots.push(slot_name);
            }
        }
        JSXExpression::JSXFragment(frag) => {
            for child in &frag.children {
                collect_slots_from_child(child, slots);
            }
        }
        JSXExpression::ConditionalExpression(cond) => {
            collect_slots_from_inner_expression(&cond.consequent, slots);
            collect_slots_from_inner_expression(&cond.alternate, slots);
        }
        JSXExpression::LogicalExpression(logic) => {
            // For &&/|| expressions, the JSX is typically on the right side
            collect_slots_from_inner_expression(&logic.right, slots);
        }
        JSXExpression::ParenthesizedExpression(paren) => {
            collect_slots_from_inner_expression(&paren.expression, slots);
        }
        JSXExpression::ArrowFunctionExpression(arrow) => {
            // Look for slots inside arrow function bodies (e.g., switch returning slotted JSX)
            collect_slots_from_function_body(&arrow.body, slots);
        }
        _ => {}
    }
}

/// Collect slots from an inner `Expression` (not `JSXExpression`).
fn collect_slots_from_inner_expression<'a>(expr: &'a Expression<'a>, slots: &mut Vec<&'a str>) {
    match expr {
        Expression::JSXElement(el) => {
            if let Some(slot_name) = get_slot_attribute(&el.opening_element.attributes) {
                slots.push(slot_name);
            }
        }
        Expression::JSXFragment(frag) => {
            for child in &frag.children {
                collect_slots_from_child(child, slots);
            }
        }
        Expression::ConditionalExpression(cond) => {
            collect_slots_from_inner_expression(&cond.consequent, slots);
            collect_slots_from_inner_expression(&cond.alternate, slots);
        }
        Expression::LogicalExpression(logic) => {
            collect_slots_from_inner_expression(&logic.right, slots);
        }
        Expression::ParenthesizedExpression(paren) => {
            collect_slots_from_inner_expression(&paren.expression, slots);
        }
        Expression::ArrowFunctionExpression(arrow) => {
            collect_slots_from_function_body(&arrow.body, slots);
        }
        _ => {}
    }
}

/// Collect slot names from return statements inside a function body.
fn collect_slots_from_function_body<'a>(
    body: &'a oxc_ast::ast::FunctionBody<'a>,
    slots: &mut Vec<&'a str>,
) {
    for stmt in &body.statements {
        collect_slots_from_statement(stmt, slots);
    }
}

/// Recursively collect slot names from statements (looking into return, switch, if, block).
fn collect_slots_from_statement<'a>(
    stmt: &'a oxc_ast::ast::Statement<'a>,
    slots: &mut Vec<&'a str>,
) {
    use oxc_ast::ast::Statement;
    match stmt {
        Statement::ReturnStatement(ret) => {
            if let Some(arg) = &ret.argument {
                collect_slots_from_inner_expression(arg, slots);
            }
        }
        Statement::SwitchStatement(switch_stmt) => {
            for case in &switch_stmt.cases {
                for s in &case.consequent {
                    collect_slots_from_statement(s, slots);
                }
            }
        }
        Statement::BlockStatement(block) => {
            for s in &block.body {
                collect_slots_from_statement(s, slots);
            }
        }
        Statement::IfStatement(if_stmt) => {
            collect_slots_from_statement(&if_stmt.consequent, slots);
            if let Some(alt) = &if_stmt.alternate {
                collect_slots_from_statement(alt, slots);
            }
        }
        _ => {}
    }
}

/// Collect slots from a JSX child.
fn collect_slots_from_child<'a>(child: &'a JSXChild<'a>, slots: &mut Vec<&'a str>) {
    match child {
        JSXChild::Element(el) => {
            if let Some(slot_name) = get_slot_attribute(&el.opening_element.attributes) {
                slots.push(slot_name);
            }
        }
        JSXChild::Fragment(frag) => {
            for child in &frag.children {
                collect_slots_from_child(child, slots);
            }
        }
        JSXChild::ExpressionContainer(expr) => {
            collect_slots_from_expression(&expr.expression, slots);
        }
        _ => {}
    }
}

impl<'a> AstroCodegen<'a> {
    /// Check if a JSX child has meaningful content (not just whitespace or empty expressions).
    pub(super) fn jsx_child_has_content(child: &JSXChild<'a>) -> bool {
        match child {
            JSXChild::Text(text) => !text.value.trim().is_empty(),
            JSXChild::ExpressionContainer(expr) => {
                !matches!(expr.expression, JSXExpression::EmptyExpression(_))
            }
            JSXChild::Element(_)
            | JSXChild::Fragment(_)
            | JSXChild::Spread(_)
            | JSXChild::AstroComment(_) => true,
            JSXChild::AstroScript(_) | JSXChild::AstroDoctype(_) => false,
        }
    }

    /// Print the opening of a slot render function: ` async? () => $$render\``.
    ///
    /// This is the common suffix after a slot key. After calling this, the
    /// caller should print the slot body and then close with `` \` ``.
    fn print_slot_fn_open(&mut self) {
        let async_prefix = self.get_async_prefix();
        let slot_params = self.get_slot_params();
        self.print(async_prefix);
        self.print(slot_params);
        self.print(runtime::RENDER);
        self.print("`");
    }

    /// Print all children as a single default slot, preserving slot attributes.
    /// Used for custom elements (web components) where the browser handles slots.
    pub(super) fn print_component_default_slot_only(&mut self, children: &[JSXChild<'a>]) {
        self.print("{\"default\": ");
        self.print_slot_fn_open();

        // DO NOT set skip_slot_attribute — we want to preserve slot="..." for custom elements
        for child in children {
            // Skip HTML comments in slots if configured
            if self.options.strip_slot_comments && matches!(child, JSXChild::AstroComment(_)) {
                continue;
            }
            self.print_jsx_child(child);
        }

        self.print("`,}");
    }

    /// Categorize children into named/default/dynamic/conditional slots and print them.
    pub(super) fn print_component_slots(&mut self, children: &[JSXChild<'a>]) {
        // Categorize children into:
        // 1. default_children — children without slot attribute
        // 2. named_slots — direct elements with slot="name"
        // 3. expression_slots — expressions containing single slotted element
        // 4. conditional_slots — expressions with multiple different slots (need $$mergeSlots)
        let mut default_children: Vec<&JSXChild<'a>> = Vec::new();
        let mut named_slots: Vec<(String, Vec<&JSXChild<'a>>)> = Vec::new();
        let mut expression_slots: Vec<(&str, &JSXChild<'a>)> = Vec::new();
        let mut conditional_slots: Vec<&JSXExpressionContainer<'a>> = Vec::new();

        // Also track dynamic slots separately (elements with slot={expr})
        let mut dynamic_slots: Vec<(String, oxc_span::Span, Vec<&JSXChild<'a>>)> = Vec::new();

        for child in children {
            // Skip HTML comments in slots if configured
            if self.options.strip_slot_comments && matches!(child, JSXChild::AstroComment(_)) {
                continue;
            }

            match child {
                JSXChild::Element(el) => {
                    // Check for slot attribute on direct element children
                    match get_slot_attribute_value(&el.opening_element.attributes) {
                        Some(SlotValue::Static(slot_name)) => {
                            // Static slot: slot="name"
                            if let Some((_, slot_children)) =
                                named_slots.iter_mut().find(|(name, _)| name == &slot_name)
                            {
                                slot_children.push(child);
                            } else {
                                named_slots.push((slot_name, vec![child]));
                            }
                        }
                        Some(SlotValue::Dynamic(expr, span)) => {
                            // Dynamic slot: slot={expr}
                            dynamic_slots.push((expr, span, vec![child]));
                        }
                        None => {
                            default_children.push(child);
                        }
                    }
                }
                JSXChild::ExpressionContainer(expr) => {
                    // Check if expression contains slotted elements
                    match extract_slots_from_expression(&expr.expression) {
                        ExpressionSlotInfo::None => {
                            default_children.push(child);
                        }
                        ExpressionSlotInfo::Single(slot_name) => {
                            expression_slots.push((slot_name, child));
                        }
                        ExpressionSlotInfo::Multiple => {
                            conditional_slots.push(expr);
                        }
                    }
                }
                _ => {
                    default_children.push(child);
                }
            }
        }

        // Determine if we need $$mergeSlots wrapper
        let needs_merge_slots = !conditional_slots.is_empty();

        if needs_merge_slots {
            self.print(runtime::MERGE_SLOTS);
            self.print("(");
        }

        self.print("{");

        // Print default slot only if there are children with actual content
        let has_meaningful_content = default_children
            .iter()
            .any(|c| Self::jsx_child_has_content(c));
        if has_meaningful_content {
            self.print("\"default\": ");
            self.print_slot_fn_open();
            for child in &default_children {
                self.print_jsx_child(child);
            }
            self.print("`,");
        }

        // Print named slots (direct elements with slot attribute)
        for (name, slot_children) in &named_slots {
            self.print("\"");
            self.print(&escape_double_quotes(name));
            self.print("\": ");
            self.print_slot_fn_open();
            // Skip slot attribute when printing these children
            self.skip_slot_attribute = true;
            for child in slot_children {
                self.print_jsx_child(child);
            }
            self.skip_slot_attribute = false;
            self.print("`,");
        }

        // Print expression slots (expressions with single slot)
        for (name, child) in &expression_slots {
            self.print("\"");
            self.print(&escape_double_quotes(name));
            self.print("\": ");
            self.print_slot_fn_open();
            self.skip_slot_attribute = true;
            self.print_jsx_child(child);
            self.skip_slot_attribute = false;
            self.print("`,");
        }

        // Print dynamic slots (elements with slot={expr}) using computed property syntax
        for (expr, span, slot_children) in &dynamic_slots {
            self.add_source_mapping_for_span(*span);
            self.print("[");
            self.print(expr);
            self.print("]: ");
            self.print_slot_fn_open();
            self.skip_slot_attribute = true;
            for child in slot_children {
                self.print_jsx_child(child);
            }
            self.skip_slot_attribute = false;
            self.print("`,");
        }

        self.print("}");

        // Print conditional slots (expressions with multiple slots) for $$mergeSlots
        for expr in &conditional_slots {
            self.print(",");
            self.print_conditional_slot_expression(expr);
        }

        if needs_merge_slots {
            self.print(")");
        }
    }

    /// Print an expression with multiple conditional slots for `$$mergeSlots`.
    ///
    /// Transforms: `cond ? <div slot="a"> : <div slot="b">`
    /// Into: `cond ? {"a": () => $$render`<div>`} : {"b": () => $$render`<div>`}`
    fn print_conditional_slot_expression(&mut self, expr: &JSXExpressionContainer<'a>) {
        self.print_conditional_slot_expr(&expr.expression);
    }

    fn print_conditional_slot_expr(&mut self, expr: &JSXExpression<'a>) {
        match expr {
            JSXExpression::ConditionalExpression(cond) => {
                self.add_source_mapping_for_span(cond.span);
                self.print_expression(&cond.test);
                self.print(" ? ");
                self.print_conditional_slot_branch(&cond.consequent);
                self.print(" : ");
                self.print_conditional_slot_branch(&cond.alternate);
            }
            JSXExpression::ArrowFunctionExpression(arrow) => {
                self.add_source_mapping_for_span(arrow.span);
                self.print_slot_aware_arrow_function(arrow);
            }
            _ => {
                if let Some(inner) = expr.as_expression() {
                    self.print_conditional_slot_branch_expr(inner);
                } else {
                    self.print_jsx_expression(expr);
                }
            }
        }
    }

    /// Print an expression that might contain conditional slot returns.
    fn print_conditional_slot_branch_expr(&mut self, expr: &Expression<'a>) {
        match expr {
            Expression::ArrowFunctionExpression(arrow) => {
                self.add_source_mapping_for_span(arrow.span);
                self.print_slot_aware_arrow_function(arrow);
            }
            Expression::ConditionalExpression(cond) => {
                self.add_source_mapping_for_span(cond.span);
                self.print_expression(&cond.test);
                self.print(" ? ");
                self.print_conditional_slot_branch(&cond.consequent);
                self.print(" : ");
                self.print_conditional_slot_branch(&cond.alternate);
            }
            _ => {
                self.print_expression(expr);
            }
        }
    }

    /// Print an arrow function where return statements may contain slotted JSX.
    ///
    /// Transforms `return <div slot="a">A</div>` into
    /// `return {"a": () => $$render\`<div>A</div>\`}`
    fn print_slot_aware_arrow_function(
        &mut self,
        arrow: &oxc_ast::ast::ArrowFunctionExpression<'a>,
    ) {
        self.add_source_mapping_for_span(arrow.span);
        self.print_arrow_params(arrow);

        if arrow.expression {
            // Expression body
            if let Some(expr) = arrow.body.statements.first()
                && let oxc_ast::ast::Statement::ExpressionStatement(expr_stmt) = expr
            {
                self.print_conditional_slot_branch(&expr_stmt.expression);
            }
        } else {
            // Block body with slot-aware statements
            self.print("{\n");
            for stmt in &arrow.body.statements {
                self.print_slot_aware_statement(stmt);
            }
            self.print("}");
        }
    }

    /// Print a statement where return values may contain slotted JSX.
    fn print_slot_aware_statement(&mut self, stmt: &oxc_ast::ast::Statement<'a>) {
        use oxc_ast::ast::Statement;
        match stmt {
            Statement::ReturnStatement(ret) => {
                self.add_source_mapping_for_span(ret.span);
                self.print("return ");
                if let Some(arg) = &ret.argument {
                    self.print_conditional_slot_branch(arg);
                }
                self.print("\n");
            }
            Statement::SwitchStatement(switch_stmt) => {
                self.add_source_mapping_for_span(switch_stmt.span);
                self.print("switch (");
                self.print_expression(&switch_stmt.discriminant);
                self.print(") {\n");
                for case in &switch_stmt.cases {
                    self.add_source_mapping_for_span(case.span);
                    if let Some(test) = &case.test {
                        self.print("case ");
                        self.print_expression(test);
                        self.print(":");
                    } else {
                        self.print("default:");
                    }
                    for s in &case.consequent {
                        self.print_slot_aware_statement(s);
                    }
                    self.print("\n");
                }
                self.print("}");
            }
            Statement::BlockStatement(block) => {
                self.add_source_mapping_for_span(block.span);
                self.print("{\n");
                for s in &block.body {
                    self.print_slot_aware_statement(s);
                }
                self.print("}");
            }
            Statement::IfStatement(if_stmt) => {
                self.add_source_mapping_for_span(if_stmt.span);
                self.print("if (");
                self.print_expression(&if_stmt.test);
                self.print(") ");
                self.print_slot_aware_statement(&if_stmt.consequent);
                if let Some(alt) = &if_stmt.alternate {
                    self.print(" else ");
                    self.print_slot_aware_statement(alt);
                }
            }
            _ => {
                self.add_source_mapping_for_span(stmt.span());
                let code = gen_to_string(stmt);
                self.print(&code);
                self.print("\n");
            }
        }
    }

    pub(super) fn print_conditional_slot_branch(&mut self, expr: &Expression<'a>) {
        match expr {
            Expression::JSXElement(el) => {
                self.add_source_mapping_for_span(el.span);
                // Extract slot name
                if let Some(slot_name) = get_slot_attribute(&el.opening_element.attributes) {
                    self.print("{\"");
                    self.print(&escape_double_quotes(slot_name));
                    self.print("\": ");
                    self.print_slot_fn_open();
                    self.skip_slot_attribute = true;
                    self.print_jsx_element(el);
                    self.skip_slot_attribute = false;
                    self.print("`}");
                } else {
                    // No slot attribute — print as default
                    self.print("{\"default\": ");
                    self.print_slot_fn_open();
                    self.print_jsx_element(el);
                    self.print("`}");
                }
            }
            Expression::ConditionalExpression(cond) => {
                // Nested ternary
                self.add_source_mapping_for_span(cond.span);
                self.print_expression(&cond.test);
                self.print(" ? ");
                self.print_conditional_slot_branch(&cond.consequent);
                self.print(" : ");
                self.print_conditional_slot_branch(&cond.alternate);
            }
            _ => {
                // Other expression types — use default codegen
                self.print_expression(expr);
            }
        }
    }
}
