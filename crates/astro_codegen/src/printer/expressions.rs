//! JavaScript and JSX expression printing.
//!
//! Contains `impl AstroCodegen` methods for rendering JS expressions that may
//! contain JSX. This includes ternary/logical expressions, arrow functions,
//! call expressions, optional chaining, and JSX fragments — any case where
//! JavaScript expressions and JSX are interleaved.

use oxc_ast::ast::*;
use oxc_codegen::{Codegen, Context, Gen, GenExpr};

use super::AstroCodegen;
use super::runtime;

impl<'a> AstroCodegen<'a> {
    pub(super) fn print_jsx_fragment(&mut self, frag: &JSXFragment<'a>) {
        // Render fragment using $$renderComponent with Fragment
        let async_prefix = self.get_async_prefix();
        let slot_params = self.get_slot_params();
        self.print("${");
        self.print(runtime::RENDER_COMPONENT);
        self.print("(");
        self.print(runtime::RESULT);
        self.print(",\"Fragment\",");
        self.print(runtime::FRAGMENT);
        self.print(&format!(",{{}},{{\"default\": {async_prefix}{slot_params}"));
        self.print(runtime::RENDER);
        self.print("`");
        for child in &frag.children {
            self.print_jsx_child(child);
        }
        self.print("`,})}");
    }

    pub(super) fn print_jsx_expression_container(&mut self, expr: &JSXExpressionContainer<'a>) {
        // Check if this is a comment-only expression that should be stripped
        if let JSXExpression::EmptyExpression(empty) = &expr.expression {
            let content = &self.source_text[empty.span.start as usize..empty.span.end as usize];
            let trimmed = content.trim();
            // Comment-only expressions should be stripped entirely
            if !trimmed.is_empty() {
                return;
            }
        }

        self.print("${");
        self.print_jsx_expression(&expr.expression);
        self.print("}");
    }

    pub(super) fn print_jsx_expression(&mut self, expr: &JSXExpression<'a>) {
        match expr {
            JSXExpression::EmptyExpression(_) => {
                // Empty {} or whitespace-only {   } renders as (void 0)
                // Comment-only expressions are already filtered out in print_jsx_expression_container
                self.print("(void 0)");
            }
            JSXExpression::Identifier(ident) => {
                self.print(ident.name.as_str());
            }
            other => {
                // Use regular codegen for complex expressions
                if let Some(expr) = other.as_expression() {
                    self.print_expression(expr);
                }
            }
        }
    }

    pub(super) fn print_jsx_spread_child(&mut self, spread: &JSXSpreadChild<'a>) {
        self.print("${");
        self.print_expression(&spread.expression);
        self.print("}");
    }

    pub(super) fn print_expression(&mut self, expr: &Expression<'a>) {
        // Handle expressions that may contain JSX
        // JSX inside expressions needs to be wrapped in $$render`...`
        match expr {
            Expression::JSXElement(el) => {
                // Wrap JSX in $$render`...`
                self.print(runtime::RENDER);
                self.print("`");
                self.print_jsx_element(el);
                self.print("`");
            }
            Expression::JSXFragment(frag) => {
                // Check if this is an explicit fragment (<>...</>) or implicit (multiple JSX siblings)
                // Implicit fragments have zero-length opening_fragment span
                let is_explicit_fragment = !frag.opening_fragment.span.is_empty();

                if is_explicit_fragment {
                    // Explicit <>...</> syntax gets wrapped in $$renderComponent with Fragment
                    let slot_params = self.get_slot_params();
                    self.print(runtime::RENDER);
                    self.print("`${");
                    self.print(runtime::RENDER_COMPONENT);
                    self.print(&format!(
                        "($$result,\"Fragment\",Fragment,{{}},{{\"default\":{slot_params}"
                    ));
                    self.print(runtime::RENDER);
                    self.print("`");
                    for child in &frag.children {
                        self.print_jsx_child(child);
                    }
                    self.print("`,})}`");
                } else {
                    // Implicit fragments (multiple JSX siblings) are just wrapped in $$render`...`
                    self.print(runtime::RENDER);
                    self.print("`");
                    for child in &frag.children {
                        self.print_jsx_child(child);
                    }
                    self.print("`");
                }
            }
            Expression::ConditionalExpression(cond) => {
                // Recursively handle ternary: test ? consequent : alternate
                self.print_expression(&cond.test);
                self.print(" ? ");
                self.print_expression(&cond.consequent);
                self.print(" : ");
                self.print_expression(&cond.alternate);
            }
            Expression::LogicalExpression(logic) => {
                // Recursively handle && and ||
                self.print_expression(&logic.left);
                self.print(match logic.operator {
                    oxc_ast::ast::LogicalOperator::And => " && ",
                    oxc_ast::ast::LogicalOperator::Or => " || ",
                    oxc_ast::ast::LogicalOperator::Coalesce => " ?? ",
                });
                self.print_expression(&logic.right);
            }
            Expression::ParenthesizedExpression(paren) => {
                self.print("(");
                self.print_expression(&paren.expression);
                self.print(")");
            }
            Expression::ChainExpression(chain) => {
                // Handle optional chaining like arr?.map(x => <JSX>)
                self.print_chain_expression(chain);
            }
            Expression::CallExpression(call) => {
                // Handle call expressions like arr.map(x => <JSX>)
                self.print_call_expression(call);
            }
            Expression::ArrowFunctionExpression(arrow) => {
                // Handle arrow functions that may return JSX
                self.print_arrow_function(arrow);
            }
            _ => {
                // For all other expressions, use regular codegen
                let mut codegen = Codegen::new();
                expr.print_expr(
                    &mut codegen,
                    oxc_syntax::precedence::Precedence::Lowest,
                    Context::default().with_typescript(),
                );
                let code = codegen.into_source_text();
                self.print(&code);
            }
        }
    }

    fn print_chain_expression(&mut self, chain: &oxc_ast::ast::ChainExpression<'a>) {
        match &chain.expression {
            oxc_ast::ast::ChainElement::CallExpression(call) => {
                self.print_call_expression(call);
            }
            oxc_ast::ast::ChainElement::StaticMemberExpression(member) => {
                self.print_expression(&member.object);
                self.print(if member.optional { "?." } else { "." });
                self.print(member.property.name.as_str());
            }
            oxc_ast::ast::ChainElement::ComputedMemberExpression(member) => {
                self.print_expression(&member.object);
                self.print(if member.optional { "?.[" } else { "[" });
                self.print_expression(&member.expression);
                self.print("]");
            }
            _ => {
                // TSNonNullExpression, PrivateFieldExpression — use source text fallback
                let start = chain.span.start as usize;
                let end = chain.span.end as usize;
                if start < self.source_text.len() && end <= self.source_text.len() {
                    self.print(&self.source_text[start..end]);
                }
            }
        }
    }

    pub(super) fn print_call_expression(&mut self, call: &oxc_ast::ast::CallExpression<'a>) {
        // Print callee
        match &call.callee {
            Expression::StaticMemberExpression(member) => {
                self.print_expression(&member.object);
                self.print(if member.optional { "?." } else { "." });
                self.print(member.property.name.as_str());
            }
            Expression::ComputedMemberExpression(member) => {
                self.print_expression(&member.object);
                self.print(if member.optional { "?.[" } else { "[" });
                self.print_expression(&member.expression);
                self.print("]");
            }
            other => {
                self.print_expression(other);
            }
        }
        // Print optional call syntax
        if call.optional {
            self.print("?.");
        }
        // Print arguments
        self.print("(");
        let mut first = true;
        for arg in &call.arguments {
            if !first {
                self.print(", ");
            }
            first = false;
            match arg {
                oxc_ast::ast::Argument::SpreadElement(spread) => {
                    self.print("...");
                    self.print_expression(&spread.argument);
                }
                _ => {
                    if let Some(expr) = arg.as_expression() {
                        self.print_expression(expr);
                    }
                }
            }
        }
        self.print(")");
    }

    pub(super) fn print_arrow_function(
        &mut self,
        arrow: &oxc_ast::ast::ArrowFunctionExpression<'a>,
    ) {
        if arrow.r#async {
            self.print("async ");
        }
        // Print parameters
        // Single simple identifier param doesn't need parens, but destructuring patterns do
        let needs_parens = arrow.params.items.len() != 1
            || arrow.params.rest.is_some()
            || !matches!(
                arrow.params.items.first().map(|p| &p.pattern),
                Some(oxc_ast::ast::BindingPattern::BindingIdentifier(_))
            );

        if needs_parens {
            self.print("(");
        }

        let mut first = true;
        for param in &arrow.params.items {
            if !first {
                self.print(", ");
            }
            first = false;
            self.print_binding_pattern(&param.pattern);
        }
        if let Some(rest) = &arrow.params.rest {
            if !first {
                self.print(", ");
            }
            self.print("...");
            self.print_binding_pattern(&rest.rest.argument);
        }

        if needs_parens {
            self.print(")");
        }
        self.print(" => ");

        // Print body
        if arrow.expression {
            // Expression body — may contain JSX
            if let Some(expr) = arrow.body.statements.first()
                && let oxc_ast::ast::Statement::ExpressionStatement(expr_stmt) = expr
            {
                self.print_expression(&expr_stmt.expression);
            }
        } else {
            // Block body — need to handle JSX in return statements
            self.print_jsx_aware_function_body(&arrow.body);
        }
    }

    fn print_jsx_aware_function_body(&mut self, body: &oxc_ast::ast::FunctionBody<'a>) {
        self.print("{\n");
        for stmt in &body.statements {
            self.print_jsx_aware_statement(stmt);
        }
        self.print("}");
    }

    fn print_jsx_aware_statement(&mut self, stmt: &oxc_ast::ast::Statement<'a>) {
        use oxc_ast::ast::Statement;
        match stmt {
            Statement::ReturnStatement(ret) => {
                self.print("\treturn ");
                if let Some(arg) = &ret.argument {
                    self.print_expression(arg);
                }
                self.print(";\n");
            }
            Statement::ExpressionStatement(expr_stmt) => {
                self.print_expression(&expr_stmt.expression);
                self.print(";\n");
            }
            Statement::VariableDeclaration(decl) => {
                // Use regular codegen for variable declarations
                let mut codegen = Codegen::new();
                decl.print(&mut codegen, Context::default().with_typescript());
                let code = codegen.into_source_text();
                self.print(&code);
                self.print("\n");
            }
            Statement::IfStatement(if_stmt) => {
                self.print("if (");
                self.print_expression(&if_stmt.test);
                self.print(") ");
                self.print_jsx_aware_statement(&if_stmt.consequent);
                if let Some(alt) = &if_stmt.alternate {
                    self.print(" else ");
                    self.print_jsx_aware_statement(alt);
                }
            }
            Statement::BlockStatement(block) => {
                self.print("{\n");
                for s in &block.body {
                    self.print_jsx_aware_statement(s);
                }
                self.print("}");
            }
            Statement::SwitchStatement(switch_stmt) => {
                self.print("switch (");
                self.print_expression(&switch_stmt.discriminant);
                self.print(") {\n");
                for case in &switch_stmt.cases {
                    if let Some(test) = &case.test {
                        self.print("case ");
                        self.print_expression(test);
                        self.print(":");
                    } else {
                        self.print("default:");
                    }
                    for s in &case.consequent {
                        self.print_jsx_aware_statement(s);
                    }
                    self.print("\n");
                }
                self.print("}");
            }
            _ => {
                // For other statements, use regular codegen
                let mut codegen = Codegen::new();
                stmt.print(&mut codegen, Context::default().with_typescript());
                let code = codegen.into_source_text();
                self.print(&code);
                self.print("\n");
            }
        }
    }

    pub(super) fn print_binding_pattern(&mut self, pattern: &oxc_ast::ast::BindingPattern<'a>) {
        if let oxc_ast::ast::BindingPattern::BindingIdentifier(ident) = pattern {
            self.print(ident.name.as_str());
        } else {
            // For complex patterns, use regular codegen
            let mut codegen = Codegen::new();
            pattern.print(&mut codegen, Context::default().with_typescript());
            let code = codegen.into_source_text();
            self.print(&code);
        }
    }
}
