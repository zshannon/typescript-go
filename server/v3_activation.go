package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/scanner"
	"go.opentelemetry.io/otel/attribute"
)

var contextHookNames = map[string]struct{}{
	"useContextAction":      {},
	"useContextObservation": {},
	"useContextState":       {},
}

type ActivationResult struct {
	Explanation ActivationExplanation `json:"explanation"`
	Manifest    ActivationManifest    `json:"manifest"`
}

type ActivationExplanation struct {
	ActivationScope string                `json:"activationScope"`
	References      []ActivationReference `json:"references"`
	Summary         string                `json:"summary"`
}

type ActivationManifest struct {
	Contexts     map[string]map[string]ActivationRequirement `json:"contexts"`
	Dependencies map[string]string                           `json:"dependencies"`
}

type ActivationReference struct {
	Context  string         `json:"context"`
	Hook     string         `json:"hook"`
	Message  string         `json:"message"`
	Required bool           `json:"required"`
	Span     ActivationSpan `json:"span"`
}

type ActivationRequirement struct {
	Required bool `json:"required"`
}

type ActivationPosition struct {
	Column int `json:"column"`
	Line   int `json:"line"`
}

type ActivationSpan struct {
	End   ActivationPosition `json:"end"`
	File  string             `json:"file"`
	Start ActivationPosition `json:"start"`
}

type activationAnalyzer struct {
	activeCallables    map[*ast.Node]struct{}
	bindings           *activationBindingFrame
	checker            *checker.Checker
	context            context.Context
	contract           *hostContract
	errors             []DiagnosticErrorV2
	recordedReferences map[activationReferenceVisit]struct{}
	references         []ActivationReference
	visitedCallables   map[*ast.Node]struct{}
	visitedCalls       map[activationCallVisit]struct{}
	visitedReferences  map[*ast.Symbol]struct{}
}

type activationBindingFrame struct {
	parent *activationBindingFrame
	values map[*ast.Symbol]*ast.Node
}

type activationCallVisit struct {
	call  *ast.Node
	frame *activationBindingFrame
}

type activationReferenceVisit struct {
	call     *ast.Node
	context  string
	hook     string
	required bool
}

func deriveActivation(ctx context.Context, program *compiler.Program, entryPoint string, hostContext *hostCompilationContext) (result ActivationResult, diagnostics []DiagnosticErrorV2) {
	hostPresent := hostContext != nil && hostContext.contract != nil
	overridePresent := hostContext != nil && hostContext.overrideScope != ""
	ctx, span := startSpan(ctx, "fly_tsgo.activation.derive",
		attribute.String("fly_tsgo.activation.entry_point", normalizeV3EntryPoint(entryPoint)),
		attribute.Bool("fly_tsgo.activation.host.present", hostPresent),
		attribute.Bool("fly_tsgo.activation.override.present", overridePresent),
	)
	errorSlug := ""
	referenceCount := 0
	requiredReferenceCount := 0
	var compilerTrace *otelCompilerTracer
	defer func() {
		if compilerTrace != nil {
			compilerTrace.flush(ctx)
		}
		span.SetAttributes(
			attribute.Int("fly_tsgo.activation.errors.count", len(diagnostics)),
			attribute.Int("fly_tsgo.activation.references.count", referenceCount),
			attribute.Int("fly_tsgo.activation.required_references.count", requiredReferenceCount),
			attribute.Bool("fly_tsgo.activation.success", len(diagnostics) == 0),
		)
		if hostPresent {
			span.SetAttributes(attribute.String("fly_tsgo.activation.provider", hostContext.contract.manifest.Name))
		}
		if result.Explanation.ActivationScope != "" {
			span.SetAttributes(attribute.String("fly_tsgo.activation.scope", result.Explanation.ActivationScope))
		}
		if len(diagnostics) > 0 {
			if errorSlug == "" {
				errorSlug = "err-activation-derive"
			}
			recordSpanError(span, errorSlug, errors.New(diagnostics[0].Message))
		}
		span.End()
	}()

	if program == nil || hostContext == nil || hostContext.contract == nil {
		errorSlug = "err-activation-precondition"
		return ActivationResult{}, []DiagnosticErrorV2{{Message: "activation analysis requires a typechecked host compilation"}}
	}
	if tracer, ok := program.Tracing().(*otelCompilerTracer); ok {
		compilerTrace = tracer
		compilerTrace.setContext(ctx)
	}

	entryPoint = normalizeV3EntryPoint(entryPoint)
	sourceFile := program.GetSourceFile(entryPoint)
	if sourceFile == nil {
		errorSlug = "err-activation-entry-point"
		return ActivationResult{}, []DiagnosticErrorV2{{
			File:    entryPoint,
			Message: "activation analysis could not find the package entry point",
		}}
	}

	typeChecker, done := program.GetTypeChecker(ctx)
	defer done()

	analyzer := &activationAnalyzer{
		activeCallables:    make(map[*ast.Node]struct{}),
		checker:            typeChecker,
		context:            ctx,
		contract:           hostContext.contract,
		recordedReferences: make(map[activationReferenceVisit]struct{}),
		references:         make([]ActivationReference, 0),
		visitedCallables:   make(map[*ast.Node]struct{}),
		visitedCalls:       make(map[activationCallVisit]struct{}),
		visitedReferences:  make(map[*ast.Symbol]struct{}),
	}
	analyzer.visitEntryPoint(sourceFile)
	referenceCount = len(analyzer.references)
	for _, reference := range analyzer.references {
		if reference.Required {
			requiredReferenceCount++
		}
	}
	if err := ctx.Err(); err != nil {
		errorSlug = "err-activation-cancelled"
		return ActivationResult{}, []DiagnosticErrorV2{{Message: "activation analysis cancelled: " + err.Error()}}
	}
	if len(analyzer.errors) > 0 {
		errorSlug = "err-activation-reference"
		return ActivationResult{}, analyzer.errors
	}

	sort.Slice(analyzer.references, func(left int, right int) bool {
		a := analyzer.references[left]
		b := analyzer.references[right]
		if a.Context != b.Context {
			return a.Context < b.Context
		}
		if a.Span.File != b.Span.File {
			return a.Span.File < b.Span.File
		}
		if a.Span.Start.Line != b.Span.Start.Line {
			return a.Span.Start.Line < b.Span.Start.Line
		}
		if a.Span.Start.Column != b.Span.Start.Column {
			return a.Span.Start.Column < b.Span.Start.Column
		}
		return a.Hook < b.Hook
	})

	scope, err := analyzer.activationScope(hostContext.overrideScope)
	if err != nil {
		errorSlug = "err-activation-scope"
		return ActivationResult{}, []DiagnosticErrorV2{{Message: err.Error()}}
	}

	provider := hostContext.contract.manifest.Name
	requirements := make(map[string]ActivationRequirement)
	for _, reference := range analyzer.references {
		current, exists := requirements[reference.Context]
		requirements[reference.Context] = ActivationRequirement{
			Required: reference.Required || exists && current.Required,
		}
	}

	summary := fmt.Sprintf("Activate %s for %d host context reference", provider+"/"+scope, len(analyzer.references))
	if len(analyzer.references) != 1 {
		summary += "s"
	}

	return ActivationResult{
		Explanation: ActivationExplanation{
			ActivationScope: provider + "/" + scope,
			References:      analyzer.references,
			Summary:         summary,
		},
		Manifest: ActivationManifest{
			Contexts: map[string]map[string]ActivationRequirement{
				provider: requirements,
			},
			Dependencies: map[string]string{
				provider: "*",
			},
		},
	}, nil
}

func (analyzer *activationAnalyzer) visitEntryPoint(sourceFile *ast.SourceFile) {
	for _, statement := range sourceFile.Statements.Nodes {
		switch {
		case ast.IsFunctionDeclaration(statement) && ast.HasSyntacticModifier(statement, ast.ModifierFlagsDefault):
			analyzer.visitCallable(statement)
		case ast.IsExportAssignment(statement):
			expression := statement.Expression()
			if ast.IsFunctionLike(expression) {
				analyzer.visitCallable(expression)
			} else {
				analyzer.visitCallableValue(expression)
			}
		default:
			analyzer.visitNode(statement)
		}
	}
	analyzer.followDefaultExport(sourceFile)
}

func (analyzer *activationAnalyzer) followDefaultExport(sourceFile *ast.SourceFile) {
	moduleSymbol := sourceFile.AsNode().Symbol()
	if moduleSymbol == nil {
		return
	}
	for _, exported := range analyzer.checker.GetExportsOfModule(moduleSymbol) {
		if exported.Name != ast.InternalSymbolNameDefault {
			continue
		}
		resolved := analyzer.resolveAlias(exported)
		if resolved == nil {
			return
		}
		for _, declaration := range resolved.Declarations {
			analyzer.followDeclaration(declaration)
		}
		return
	}
}

func (analyzer *activationAnalyzer) visitNode(node *ast.Node) {
	if node == nil || analyzer.context.Err() != nil {
		return
	}
	if ast.IsFunctionLike(node) {
		return
	}

	switch {
	case ast.IsCallExpression(node):
		analyzer.visitCall(node)
	case ast.IsJsxOpeningElement(node), ast.IsJsxSelfClosingElement(node):
		analyzer.followExpression(node.TagName())
	case ast.IsJsxAttribute(node):
		analyzer.visitCallableValue(node.Initializer())
	case ast.IsReturnStatement(node):
		analyzer.visitCallableValue(node.Expression())
	case ast.IsBinaryExpression(node) && ast.IsAssignmentOperator(node.AsBinaryExpression().OperatorToken.Kind):
		analyzer.visitCallableValue(node.AsBinaryExpression().Right)
	}

	node.ForEachChild(func(child *ast.Node) bool {
		analyzer.visitNode(child)
		return false
	})
}

func (analyzer *activationAnalyzer) visitCall(call *ast.Node) {
	visit := activationCallVisit{call: call, frame: analyzer.bindings}
	if _, ok := analyzer.visitedCalls[visit]; ok {
		return
	}
	analyzer.visitedCalls[visit] = struct{}{}

	signature := analyzer.checker.GetResolvedSignature(call)
	if hookName := analyzer.hookName(signature, call.Expression()); hookName != "" {
		analyzer.recordHook(call, hookName)
		return
	}

	followedDeclaration := false
	if signature != nil {
		followedDeclaration = analyzer.visitInvokedDeclaration(signature.Declaration(), call.Arguments())
		if !followedDeclaration {
			analyzer.followDeclaration(signature.Declaration())
		}
	}
	if !followedDeclaration {
		analyzer.followExpression(call.Expression())
	}
	for _, argument := range call.Arguments() {
		analyzer.visitCallableValue(argument)
	}
}

func (analyzer *activationAnalyzer) visitInvokedDeclaration(declaration *ast.Node, arguments []*ast.Node) bool {
	declaration = analyzer.callableImplementation(declaration)
	if declaration == nil || !ast.IsFunctionLike(declaration) || !isUserDeclaration(declaration) {
		return false
	}
	if _, ok := analyzer.activeCallables[declaration]; ok {
		return true
	}

	values := make(map[*ast.Symbol]*ast.Node)
	for index, parameter := range declaration.Parameters() {
		value := parameter.Initializer()
		if index < len(arguments) {
			value = arguments[index]
		}
		if value == nil || parameter.Name() == nil {
			continue
		}
		symbol := analyzer.resolveAlias(analyzer.checker.GetSymbolAtLocation(parameter.Name()))
		if symbol != nil {
			values[symbol] = value
		}
	}

	frame := &activationBindingFrame{parent: analyzer.bindings, values: values}
	previousBindings := analyzer.bindings
	analyzer.bindings = frame
	analyzer.activeCallables[declaration] = struct{}{}
	analyzer.visitNode(declaration.Body())
	delete(analyzer.activeCallables, declaration)
	analyzer.bindings = previousBindings
	return true
}

func (analyzer *activationAnalyzer) callableImplementation(declaration *ast.Node) *ast.Node {
	if declaration == nil || declaration.Body() != nil || declaration.Name() == nil {
		return declaration
	}
	symbol := analyzer.resolveAlias(analyzer.checker.GetSymbolAtLocation(declaration.Name()))
	if symbol == nil {
		return declaration
	}
	for _, sibling := range symbol.Declarations {
		if ast.IsFunctionLike(sibling) && sibling.Body() != nil {
			return sibling
		}
	}
	return declaration
}

func (analyzer *activationAnalyzer) hookName(signature *checker.Signature, expression *ast.Node) string {
	if signature != nil {
		if name := analyzer.hookNameFromDeclaration(signature.Declaration()); name != "" {
			return name
		}
	}
	return analyzer.hookNameFromSymbol(analyzer.checker.GetSymbolAtLocation(expression), make(map[*ast.Symbol]struct{}))
}

func (analyzer *activationAnalyzer) hookNameFromDeclaration(declaration *ast.Node) string {
	if declaration == nil || declaration.Name() == nil {
		return ""
	}
	name := declaration.Name().Text()
	if _, ok := contextHookNames[name]; !ok {
		return ""
	}
	sourceFile := ast.GetSourceFileOfNode(declaration)
	if sourceFile == nil || !strings.HasPrefix(sourceFile.FileName(), "/node_modules/@flickfyi/core/") {
		return ""
	}
	return name
}

func (analyzer *activationAnalyzer) hookNameFromSymbol(symbol *ast.Symbol, visited map[*ast.Symbol]struct{}) string {
	symbol = analyzer.resolveAlias(symbol)
	if symbol == nil {
		return ""
	}
	if _, ok := visited[symbol]; ok {
		return ""
	}
	visited[symbol] = struct{}{}
	if expression := analyzer.boundExpression(symbol); expression != nil {
		if name := analyzer.hookNameFromSymbol(analyzer.checker.GetSymbolAtLocation(expression), visited); name != "" {
			return name
		}
	}

	for _, declaration := range symbol.Declarations {
		if name := analyzer.hookNameFromDeclaration(declaration); name != "" {
			return name
		}
		if ast.IsVariableDeclaration(declaration) && declaration.Initializer() != nil {
			candidate := analyzer.checker.GetSymbolAtLocation(declaration.Initializer())
			if name := analyzer.hookNameFromSymbol(candidate, visited); name != "" {
				return name
			}
		}
	}
	return ""
}

func (analyzer *activationAnalyzer) recordHook(call *ast.Node, hookName string) {
	arguments := call.Arguments()
	if len(arguments) == 0 {
		analyzer.addError(call, hookName+" requires a statically typed host address")
		return
	}

	address := analyzer.resolveBoundExpression(arguments[0])
	addressType := analyzer.checker.GetTypeAtLocation(address)
	uriType := analyzer.checker.GetTypeOfPropertyOfType(addressType, "uri")
	uri, ok := literalString(uriType)
	if !ok {
		analyzer.addError(address, hookName+" address must resolve to one literal host context URI")
		return
	}

	contextPath, ok := analyzer.contract.contextPath(uri)
	if !ok {
		analyzer.addError(address, fmt.Sprintf("%s references context outside host %s: %s", hookName, analyzer.contract.manifest.Name, uri))
		return
	}
	if _, ok := analyzer.contract.manifest.Exports[contextPath]; !ok || !strings.Contains(contextPath, "#") {
		analyzer.addError(address, fmt.Sprintf("%s references an unexported host capability: %s", hookName, uri))
		return
	}
	if !hookSupportsContext(hookName, contextPath) {
		analyzer.addError(address, fmt.Sprintf("%s cannot consume host capability %s", hookName, uri))
		return
	}

	required := true
	if len(arguments) > 1 {
		options := analyzer.resolveBoundExpression(arguments[1])
		optionsType := analyzer.checker.GetTypeAtLocation(options)
		optionalType := analyzer.checker.GetTypeOfPropertyOfType(optionsType, "optional")
		if optionalType != nil {
			optional, literal := literalBoolean(optionalType)
			if !literal {
				analyzer.addError(options, hookName+" optional flag must resolve to the literal true or false")
				return
			}
			required = !optional
		}
	}

	message := "Requires " + uri
	if !required {
		message = "Optionally uses " + uri
	}
	visit := activationReferenceVisit{
		call:     call,
		context:  contextPath,
		hook:     hookName,
		required: required,
	}
	if _, ok := analyzer.recordedReferences[visit]; ok {
		return
	}
	analyzer.recordedReferences[visit] = struct{}{}
	analyzer.references = append(analyzer.references, ActivationReference{
		Context:  contextPath,
		Hook:     hookName,
		Message:  message,
		Required: required,
		Span:     activationSpan(call),
	})
}

func (analyzer *activationAnalyzer) activationScope(override string) (string, error) {
	root := analyzer.contract.rootScope()
	requiredScopes := make([]string, 0, len(analyzer.references))
	for _, reference := range analyzer.references {
		if reference.Required {
			requiredScopes = append(requiredScopes, strings.SplitN(reference.Context, "#", 2)[0])
		}
	}

	if override != "" {
		for _, required := range requiredScopes {
			if !isSameOrDescendantScope(required, override) {
				return "", fmt.Errorf("activation override %s does not include required scope %s", override, required)
			}
		}
		return override, nil
	}

	scope := root
	for _, required := range requiredScopes {
		switch {
		case isSameOrDescendantScope(required, scope):
			scope = required
		case isSameOrDescendantScope(scope, required):
		default:
			return "", fmt.Errorf("required host contexts have ambiguous sibling activation scopes: %s and %s", scope, required)
		}
	}

	return scope, nil
}

func (analyzer *activationAnalyzer) followExpression(expression *ast.Node) {
	if expression == nil {
		return
	}
	if bound := analyzer.resolveBoundExpression(expression); bound != expression {
		analyzer.visitCallableValue(bound)
		return
	}
	symbol := analyzer.resolveAlias(analyzer.checker.GetSymbolAtLocation(expression))
	if symbol == nil {
		return
	}
	if _, ok := analyzer.visitedReferences[symbol]; ok {
		return
	}
	analyzer.visitedReferences[symbol] = struct{}{}
	for _, declaration := range symbol.Declarations {
		analyzer.followDeclaration(declaration)
	}
}

func (analyzer *activationAnalyzer) boundExpression(symbol *ast.Symbol) *ast.Node {
	symbol = analyzer.resolveAlias(symbol)
	if symbol == nil {
		return nil
	}
	for frame := analyzer.bindings; frame != nil; frame = frame.parent {
		if expression, ok := frame.values[symbol]; ok {
			return expression
		}
	}
	return nil
}

func (analyzer *activationAnalyzer) resolveBoundExpression(expression *ast.Node) *ast.Node {
	visited := make(map[*ast.Symbol]struct{})
	for expression != nil {
		symbol := analyzer.resolveAlias(analyzer.checker.GetSymbolAtLocation(expression))
		if symbol == nil {
			return expression
		}
		if _, ok := visited[symbol]; ok {
			return expression
		}
		visited[symbol] = struct{}{}
		bound := analyzer.boundExpression(symbol)
		if bound == nil {
			return expression
		}
		expression = bound
	}
	return nil
}

func (analyzer *activationAnalyzer) followDeclaration(declaration *ast.Node) {
	if declaration == nil || !isUserDeclaration(declaration) {
		return
	}
	switch {
	case ast.IsFunctionLike(declaration):
		analyzer.visitCallable(declaration)
	case ast.IsClassLike(declaration):
		analyzer.visitCallableValue(declaration)
	case ast.IsVariableDeclaration(declaration), ast.IsPropertyAssignment(declaration), ast.IsPropertyDeclaration(declaration):
		analyzer.visitCallableValue(declaration.Initializer())
	case ast.IsShorthandPropertyAssignment(declaration):
		valueSymbol := analyzer.resolveAlias(analyzer.checker.GetShorthandAssignmentValueSymbol(declaration))
		if valueSymbol == nil {
			return
		}
		for _, valueDeclaration := range valueSymbol.Declarations {
			analyzer.followDeclaration(valueDeclaration)
		}
	case ast.IsBindingElement(declaration):
		for parent := declaration.Parent; parent != nil; parent = parent.Parent {
			if ast.IsVariableDeclaration(parent) {
				analyzer.visitCallableValue(parent.Initializer())
				return
			}
			if ast.IsFunctionLike(parent) || ast.IsSourceFile(parent) {
				return
			}
		}
	default:
		if declaration.Body() != nil {
			analyzer.visitCallable(declaration)
		}
	}
}

func (analyzer *activationAnalyzer) visitCallableValue(node *ast.Node) {
	if node == nil || analyzer.context.Err() != nil {
		return
	}
	if ast.IsFunctionLike(node) {
		analyzer.visitCallable(node)
		return
	}
	switch {
	case ast.IsCallExpression(node):
		analyzer.visitCall(node)
	case ast.IsJsxOpeningElement(node), ast.IsJsxSelfClosingElement(node):
		analyzer.followExpression(node.TagName())
	}
	analyzer.followExpression(node)
	node.ForEachChild(func(child *ast.Node) bool {
		analyzer.visitCallableValue(child)
		return false
	})
}
func (analyzer *activationAnalyzer) visitCallable(declaration *ast.Node) {
	if declaration == nil {
		return
	}
	if _, ok := analyzer.visitedCallables[declaration]; ok {
		return
	}
	analyzer.visitedCallables[declaration] = struct{}{}

	body := declaration.Body()
	if body == nil {
		name := declaration.Name()
		if name == nil {
			return
		}
		symbol := analyzer.resolveAlias(analyzer.checker.GetSymbolAtLocation(name))
		if symbol != nil {
			for _, sibling := range symbol.Declarations {
				if sibling != declaration {
					analyzer.visitCallable(sibling)
				}
			}
		}
		return
	}
	analyzer.visitNode(body)
}

func (analyzer *activationAnalyzer) resolveAlias(symbol *ast.Symbol) *ast.Symbol {
	if symbol == nil {
		return nil
	}
	if symbol.Flags&ast.SymbolFlagsAlias != 0 {
		resolved := analyzer.checker.GetAliasedSymbol(symbol)
		if resolved != nil && !analyzer.checker.IsUnknownSymbol(resolved) {
			return resolved
		}
	}
	return symbol
}

func (analyzer *activationAnalyzer) addError(node *ast.Node, message string) {
	span := activationSpan(node)
	analyzer.errors = append(analyzer.errors, DiagnosticErrorV2{
		Column:  span.Start.Column,
		File:    span.File,
		Line:    span.Start.Line,
		Message: message,
	})
}

func (contract *hostContract) contextPath(uri string) (string, bool) {
	return strings.CutPrefix(uri, contract.manifest.Name+"/")
}

func (contract *hostContract) rootScope() string {
	root := ""
	for contextPath := range contract.manifest.Exports {
		if strings.Contains(contextPath, "#") {
			continue
		}
		if root == "" || scopeDepth(contextPath) < scopeDepth(root) {
			root = contextPath
		}
	}
	return root
}

func hookSupportsContext(hookName string, contextPath string) bool {
	fragment := strings.SplitN(contextPath, "#", 2)[1]
	switch hookName {
	case "useContextAction":
		return strings.HasPrefix(fragment, "actions/")
	case "useContextObservation":
		return strings.HasPrefix(fragment, "states/") || strings.HasPrefix(fragment, "streams/")
	case "useContextState":
		return strings.HasPrefix(fragment, "states/")
	default:
		return false
	}
}

func literalBoolean(value *checker.Type) (bool, bool) {
	if value == nil || value.Flags()&checker.TypeFlagsBooleanLiteral == 0 {
		return false, false
	}
	literal, ok := value.AsLiteralType().Value().(bool)
	return literal, ok
}

func literalString(value *checker.Type) (string, bool) {
	if value == nil || !value.IsStringLiteral() {
		return "", false
	}
	literal, ok := value.AsLiteralType().Value().(string)
	return literal, ok
}

func activationSpan(node *ast.Node) ActivationSpan {
	sourceFile := ast.GetSourceFileOfNode(node)
	if sourceFile == nil {
		return ActivationSpan{}
	}
	start := scanner.GetTokenPosOfNode(node, sourceFile, false)
	startLine, startColumn := calculateLineColumn(sourceFile.Text(), start)
	endLine, endColumn := calculateLineColumn(sourceFile.Text(), node.End())
	return ActivationSpan{
		End: ActivationPosition{
			Column: endColumn + 1,
			Line:   endLine + 1,
		},
		File: sourceFile.FileName(),
		Start: ActivationPosition{
			Column: startColumn + 1,
			Line:   startLine + 1,
		},
	}
}

func isUserDeclaration(declaration *ast.Node) bool {
	sourceFile := ast.GetSourceFileOfNode(declaration)
	return sourceFile != nil &&
		!sourceFile.IsDeclarationFile &&
		!strings.HasPrefix(sourceFile.FileName(), "/node_modules/")
}

func normalizeV3EntryPoint(entryPoint string) string {
	if strings.HasPrefix(entryPoint, "./") {
		return entryPoint[1:]
	}
	if strings.HasPrefix(entryPoint, "/") {
		return entryPoint
	}
	return "/" + entryPoint
}
