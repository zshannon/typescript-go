package compiler

import (
	"encoding/base64"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/binder"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/diagnostics"
	"github.com/microsoft/typescript-go/internal/outputpaths"
	"github.com/microsoft/typescript-go/internal/printer"
	"github.com/microsoft/typescript-go/internal/sourcemap"
	"github.com/microsoft/typescript-go/internal/stringutil"
	"github.com/microsoft/typescript-go/internal/transformers"
	"github.com/microsoft/typescript-go/internal/transformers/declarations"
	"github.com/microsoft/typescript-go/internal/transformers/estransforms"
	"github.com/microsoft/typescript-go/internal/transformers/jsxtransforms"
	"github.com/microsoft/typescript-go/internal/transformers/moduletransforms"
	"github.com/microsoft/typescript-go/internal/transformers/tstransforms"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/tspath"
)

type emitOnly byte

const (
	emitAll emitOnly = iota
	emitOnlyJs
	emitOnlyDts
	emitOnlyBuildInfo
)

type emitter struct {
	host               EmitHost
	emitOnly           emitOnly
	emittedFilesList   []string
	emitterDiagnostics ast.DiagnosticsCollection
	emitSkipped        bool
	sourceMapDataList  []*SourceMapEmitResult
	writer             printer.EmitTextWriter
	paths              *outputpaths.OutputPaths
	sourceFile         *ast.SourceFile
}

func (e *emitter) emit() {
	// !!! tracing
	e.emitJSFile(e.sourceFile, e.paths.JsFilePath(), e.paths.SourceMapFilePath())
	e.emitDeclarationFile(e.sourceFile, e.paths.DeclarationFilePath(), e.paths.DeclarationMapPath())
	e.emitBuildInfo(e.paths.BuildInfoPath())
}

func (e *emitter) getDeclarationTransformers(emitContext *printer.EmitContext, declarationFilePath string, declarationMapPath string) []*declarations.DeclarationTransformer {
	transform := declarations.NewDeclarationTransformer(e.host, emitContext, e.host.Options(), declarationFilePath, declarationMapPath)
	return []*declarations.DeclarationTransformer{transform}
}

func getModuleTransformer(emitContext *printer.EmitContext, options *core.CompilerOptions, resolver binder.ReferenceResolver, getEmitModuleFormatOfFile func(file ast.HasFileName) core.ModuleKind) *transformers.Transformer {
	switch options.GetEmitModuleKind() {
	case core.ModuleKindPreserve:
		// `ESModuleTransformer` contains logic for preserving CJS input syntax in `--module preserve`
		return moduletransforms.NewESModuleTransformer(emitContext, options, resolver, getEmitModuleFormatOfFile)

	case core.ModuleKindESNext,
		core.ModuleKindES2022,
		core.ModuleKindES2020,
		core.ModuleKindES2015,
		core.ModuleKindNode18,
		core.ModuleKindNode16,
		core.ModuleKindNodeNext,
		core.ModuleKindCommonJS:
		return moduletransforms.NewImpliedModuleTransformer(emitContext, options, resolver, getEmitModuleFormatOfFile)

	default:
		return moduletransforms.NewCommonJSModuleTransformer(emitContext, options, resolver, getEmitModuleFormatOfFile)
	}
}

func getScriptTransformers(emitContext *printer.EmitContext, host printer.EmitHost, sourceFile *ast.SourceFile) []*transformers.Transformer {
	var tx []*transformers.Transformer
	options := host.Options()

	// JS files don't use reference calculations as they don't do import elision, no need to calculate it
	importElisionEnabled := !options.VerbatimModuleSyntax.IsTrue() && !ast.IsInJSFile(sourceFile.AsNode())

	var emitResolver printer.EmitResolver
	var referenceResolver binder.ReferenceResolver
	if importElisionEnabled || options.GetJSXTransformEnabled() {
		emitResolver = host.GetEmitResolver()
		emitResolver.MarkLinkedReferencesRecursively(sourceFile)
		referenceResolver = emitResolver
	} else {
		referenceResolver = binder.NewReferenceResolver(options, binder.ReferenceResolverHooks{})
	}

	// transform TypeScript syntax
	{
		// erase types
		tx = append(tx, tstransforms.NewTypeEraserTransformer(emitContext, options))

		// elide imports
		if importElisionEnabled {
			tx = append(tx, tstransforms.NewImportElisionTransformer(emitContext, options, emitResolver))
		}

		// transform `enum`, `namespace`, and parameter properties
		tx = append(tx, tstransforms.NewRuntimeSyntaxTransformer(emitContext, options, referenceResolver))
	}

	// !!! transform legacy decorator syntax
	if options.GetJSXTransformEnabled() {
		tx = append(tx, jsxtransforms.NewJSXTransformer(emitContext, options, emitResolver))
	}

	downleveler := estransforms.GetESTransformer(options, emitContext)
	if downleveler != nil {
		tx = append(tx, downleveler)
	}

	// transform module syntax
	tx = append(tx, getModuleTransformer(emitContext, options, referenceResolver, host.GetEmitModuleFormatOfFile))
	return tx
}

func (e *emitter) emitJSFile(sourceFile *ast.SourceFile, jsFilePath string, sourceMapFilePath string) {
	options := e.host.Options()

	if sourceFile == nil || e.emitOnly != emitAll && e.emitOnly != emitOnlyJs || len(jsFilePath) == 0 {
		return
	}

	if options.NoEmit == core.TSTrue || e.host.IsEmitBlocked(jsFilePath) {
		return
	}

	emitContext, putEmitContext := printer.GetEmitContext()
	defer putEmitContext()

	for _, transformer := range getScriptTransformers(emitContext, e.host, sourceFile) {
		sourceFile = transformer.TransformSourceFile(sourceFile)
	}

	printerOptions := printer.PrinterOptions{
		RemoveComments:  options.RemoveComments.IsTrue(),
		NewLine:         options.NewLine,
		NoEmitHelpers:   options.NoEmitHelpers.IsTrue(),
		SourceMap:       options.SourceMap.IsTrue(),
		InlineSourceMap: options.InlineSourceMap.IsTrue(),
		InlineSources:   options.InlineSources.IsTrue(),
		// !!!
	}

	// create a printer to print the nodes
	printer := printer.NewPrinter(printerOptions, printer.PrintHandlers{
		// !!!
	}, emitContext)

	e.printSourceFile(jsFilePath, sourceMapFilePath, sourceFile, printer, shouldEmitSourceMaps(options, sourceFile))

	if e.emittedFilesList != nil {
		e.emittedFilesList = append(e.emittedFilesList, jsFilePath)
		if sourceMapFilePath != "" {
			e.emittedFilesList = append(e.emittedFilesList, sourceMapFilePath)
		}
	}
}

func (e *emitter) emitDeclarationFile(sourceFile *ast.SourceFile, declarationFilePath string, declarationMapPath string) {
	options := e.host.Options()

	if sourceFile == nil || e.emitOnly != emitAll && e.emitOnly != emitOnlyDts || len(declarationFilePath) == 0 {
		return
	}

	if options.NoEmit == core.TSTrue || e.host.IsEmitBlocked(declarationFilePath) {
		return
	}

	var diags []*ast.Diagnostic
	emitContext, putEmitContext := printer.GetEmitContext()
	defer putEmitContext()
	for _, transformer := range e.getDeclarationTransformers(emitContext, declarationFilePath, declarationMapPath) {
		sourceFile = transformer.TransformSourceFile(sourceFile)
		diags = append(diags, transformer.GetDiagnostics()...)
	}

	printerOptions := printer.PrinterOptions{
		RemoveComments:  options.RemoveComments.IsTrue(),
		NewLine:         options.NewLine,
		NoEmitHelpers:   options.NoEmitHelpers.IsTrue(),
		SourceMap:       options.DeclarationMap.IsTrue(),
		InlineSourceMap: options.InlineSourceMap.IsTrue(),
		InlineSources:   options.InlineSources.IsTrue(),
		// !!!
	}

	// create a printer to print the nodes
	printer := printer.NewPrinter(printerOptions, printer.PrintHandlers{
		// !!!
	}, emitContext)

	e.printSourceFile(declarationFilePath, declarationMapPath, sourceFile, printer, shouldEmitDeclarationSourceMaps(options, sourceFile))

	for _, elem := range diags {
		// Add declaration transform diagnostics to emit diagnostics
		e.emitterDiagnostics.Add(elem)
	}
}

func (e *emitter) emitBuildInfo(buildInfoPath string) {
	// !!!
}

func (e *emitter) printSourceFile(jsFilePath string, sourceMapFilePath string, sourceFile *ast.SourceFile, printer_ *printer.Printer, shouldEmitSourceMaps bool) bool {
	// !!! sourceMapGenerator
	options := e.host.Options()
	var sourceMapGenerator *sourcemap.Generator
	if shouldEmitSourceMaps {
		sourceMapGenerator = sourcemap.NewGenerator(
			tspath.GetBaseFileName(tspath.NormalizeSlashes(jsFilePath)),
			getSourceRoot(options),
			e.getSourceMapDirectory(options, jsFilePath, sourceFile),
			tspath.ComparePathsOptions{
				UseCaseSensitiveFileNames: e.host.UseCaseSensitiveFileNames(),
				CurrentDirectory:          e.host.GetCurrentDirectory(),
			},
		)
	}

	// !!! bundles not implemented, may be deprecated
	sourceFiles := []*ast.SourceFile{sourceFile}

	printer_.Write(sourceFile.AsNode(), sourceFile, e.writer, sourceMapGenerator)

	sourceMapUrlPos := -1
	if sourceMapGenerator != nil {
		if options.SourceMap.IsTrue() || options.InlineSourceMap.IsTrue() || options.GetAreDeclarationMapsEnabled() {
			e.sourceMapDataList = append(e.sourceMapDataList, &SourceMapEmitResult{
				InputSourceFileNames: sourceMapGenerator.Sources(),
				SourceMap:            sourceMapGenerator.RawSourceMap(),
				GeneratedFile:        jsFilePath,
			})
		}

		sourceMappingURL := e.getSourceMappingURL(
			options,
			sourceMapGenerator,
			jsFilePath,
			sourceMapFilePath,
			sourceFile,
		)

		if len(sourceMappingURL) > 0 {
			if !e.writer.IsAtStartOfLine() {
				e.writer.RawWrite(core.IfElse(options.NewLine == core.NewLineKindCRLF, "\r\n", "\n"))
			}
			sourceMapUrlPos = e.writer.GetTextPos()
			e.writer.WriteComment("//# sourceMappingURL=" + sourceMappingURL)
		}

		// Write the source map
		if len(sourceMapFilePath) > 0 {
			sourceMap := sourceMapGenerator.String()
			err := e.host.WriteFile(sourceMapFilePath, sourceMap, false /*writeByteOrderMark*/, sourceFiles, nil /*data*/)
			if err != nil {
				e.emitterDiagnostics.Add(ast.NewCompilerDiagnostic(diagnostics.Could_not_write_file_0_Colon_1, jsFilePath, err.Error()))
			}
		}
	} else {
		e.writer.WriteLine()
	}

	// Write the output file
	text := e.writer.String()
	data := &printer.WriteFileData{SourceMapUrlPos: sourceMapUrlPos} // !!! transform diagnostics
	err := e.host.WriteFile(jsFilePath, text, e.host.Options().EmitBOM.IsTrue(), sourceFiles, data)
	if err != nil {
		e.emitterDiagnostics.Add(ast.NewCompilerDiagnostic(diagnostics.Could_not_write_file_0_Colon_1, jsFilePath, err.Error()))
	}

	// Reset state
	e.writer.Clear()
	return !data.SkippedDtsWrite
}

func shouldEmitSourceMaps(mapOptions *core.CompilerOptions, sourceFile *ast.SourceFile) bool {
	return (mapOptions.SourceMap.IsTrue() || mapOptions.InlineSourceMap.IsTrue()) &&
		!tspath.FileExtensionIs(sourceFile.FileName(), tspath.ExtensionJson)
}

func shouldEmitDeclarationSourceMaps(mapOptions *core.CompilerOptions, sourceFile *ast.SourceFile) bool {
	return mapOptions.DeclarationMap.IsTrue() &&
		!tspath.FileExtensionIs(sourceFile.FileName(), tspath.ExtensionJson)
}

func getSourceRoot(mapOptions *core.CompilerOptions) string {
	// Normalize source root and make sure it has trailing "/" so that it can be used to combine paths with the
	// relative paths of the sources list in the sourcemap
	sourceRoot := tspath.NormalizeSlashes(mapOptions.SourceRoot)
	if len(sourceRoot) > 0 {
		sourceRoot = tspath.EnsureTrailingDirectorySeparator(sourceRoot)
	}
	return sourceRoot
}

func (e *emitter) getSourceMapDirectory(mapOptions *core.CompilerOptions, filePath string, sourceFile *ast.SourceFile) string {
	if len(mapOptions.SourceRoot) > 0 {
		return e.host.CommonSourceDirectory()
	}
	if len(mapOptions.MapRoot) > 0 {
		sourceMapDir := tspath.NormalizeSlashes(mapOptions.MapRoot)
		if sourceFile != nil {
			// For modules or multiple emit files the mapRoot will have directory structure like the sources
			// So if src\a.ts and src\lib\b.ts are compiled together user would be moving the maps into mapRoot\a.js.map and mapRoot\lib\b.js.map
			sourceMapDir = tspath.GetDirectoryPath(outputpaths.GetSourceFilePathInNewDir(
				sourceFile.FileName(),
				sourceMapDir,
				e.host.GetCurrentDirectory(),
				e.host.CommonSourceDirectory(),
				e.host.UseCaseSensitiveFileNames(),
			))
		}
		if tspath.GetRootLength(sourceMapDir) == 0 {
			// The relative paths are relative to the common directory
			sourceMapDir = tspath.CombinePaths(e.host.CommonSourceDirectory(), sourceMapDir)
		}
		return sourceMapDir
	}
	return tspath.GetDirectoryPath(tspath.NormalizePath(filePath))
}

func (e *emitter) getSourceMappingURL(mapOptions *core.CompilerOptions, sourceMapGenerator *sourcemap.Generator, filePath string, sourceMapFilePath string, sourceFile *ast.SourceFile) string {
	if mapOptions.InlineSourceMap.IsTrue() {
		// Encode the sourceMap into the sourceMap url
		sourceMapText := sourceMapGenerator.String()
		base64SourceMapText := base64.StdEncoding.EncodeToString([]byte(sourceMapText))
		return "data:application/json;base64," + base64SourceMapText
	}

	sourceMapFile := tspath.GetBaseFileName(tspath.NormalizeSlashes(sourceMapFilePath))
	if len(mapOptions.MapRoot) > 0 {
		sourceMapDir := tspath.NormalizeSlashes(mapOptions.MapRoot)
		if sourceFile != nil {
			// For modules or multiple emit files the mapRoot will have directory structure like the sources
			// So if src\a.ts and src\lib\b.ts are compiled together user would be moving the maps into mapRoot\a.js.map and mapRoot\lib\b.js.map
			sourceMapDir = tspath.GetDirectoryPath(outputpaths.GetSourceFilePathInNewDir(
				sourceFile.FileName(),
				sourceMapDir,
				e.host.GetCurrentDirectory(),
				e.host.CommonSourceDirectory(),
				e.host.UseCaseSensitiveFileNames(),
			))
		}
		if tspath.GetRootLength(sourceMapDir) == 0 {
			// The relative paths are relative to the common directory
			sourceMapDir = tspath.CombinePaths(e.host.CommonSourceDirectory(), sourceMapDir)
			return stringutil.EncodeURI(
				tspath.GetRelativePathToDirectoryOrUrl(
					tspath.GetDirectoryPath(tspath.NormalizePath(filePath)), // get the relative sourceMapDir path based on jsFilePath
					tspath.CombinePaths(sourceMapDir, sourceMapFile),        // this is where user expects to see sourceMap
					/*isAbsolutePathAnUrl*/ true,
					tspath.ComparePathsOptions{
						UseCaseSensitiveFileNames: e.host.UseCaseSensitiveFileNames(),
						CurrentDirectory:          e.host.GetCurrentDirectory(),
					},
				),
			)
		} else {
			return stringutil.EncodeURI(tspath.CombinePaths(sourceMapDir, sourceMapFile))
		}
	}
	return stringutil.EncodeURI(sourceMapFile)
}

type SourceFileMayBeEmittedHost interface {
	Options() *core.CompilerOptions
	GetOutputAndProjectReference(path tspath.Path) *tsoptions.OutputDtsAndProjectReference
	IsSourceFileFromExternalLibrary(file *ast.SourceFile) bool
	GetCurrentDirectory() string
	UseCaseSensitiveFileNames() bool
	SourceFiles() []*ast.SourceFile
}

func sourceFileMayBeEmitted(sourceFile *ast.SourceFile, host SourceFileMayBeEmittedHost, forceDtsEmit bool) bool {
	// TODO: move this to outputpaths?

	options := host.Options()
	// Js files are emitted only if option is enabled
	if options.NoEmitForJsFiles.IsTrue() && ast.IsSourceFileJS(sourceFile) {
		return false
	}

	// Declaration files are not emitted
	if sourceFile.IsDeclarationFile {
		return false
	}

	// Source file from node_modules are not emitted
	if host.IsSourceFileFromExternalLibrary(sourceFile) {
		return false
	}

	// forcing dts emit => file needs to be emitted
	if forceDtsEmit {
		return true
	}

	// Check other conditions for file emit
	// Source files from referenced projects are not emitted
	if host.GetOutputAndProjectReference(sourceFile.Path()) != nil {
		return false
	}

	// Any non json file should be emitted
	if !ast.IsJsonSourceFile(sourceFile) {
		return true
	}

	// Json file is not emitted if outDir is not specified
	if options.OutDir == "" {
		return false
	}

	// Otherwise if rootDir or composite config file, we know common sourceDir and can check if file would be emitted in same location
	if options.RootDir != "" || (options.Composite.IsTrue() && options.ConfigFilePath != "") {
		commonDir := tspath.GetNormalizedAbsolutePath(outputpaths.GetCommonSourceDirectory(options, func() []string { return nil }, host.GetCurrentDirectory(), host.UseCaseSensitiveFileNames()), host.GetCurrentDirectory())
		outputPath := outputpaths.GetSourceFilePathInNewDirWorker(sourceFile.FileName(), options.OutDir, host.GetCurrentDirectory(), commonDir, host.UseCaseSensitiveFileNames())
		if tspath.ComparePaths(sourceFile.FileName(), outputPath, tspath.ComparePathsOptions{
			UseCaseSensitiveFileNames: host.UseCaseSensitiveFileNames(),
			CurrentDirectory:          host.GetCurrentDirectory(),
		}) == 0 {
			return false
		}
	}

	return true
}

func getSourceFilesToEmit(host SourceFileMayBeEmittedHost, targetSourceFile *ast.SourceFile, forceDtsEmit bool) []*ast.SourceFile {
	// !!! outFile not yet implemented, may be deprecated
	var sourceFiles []*ast.SourceFile
	if targetSourceFile != nil {
		sourceFiles = []*ast.SourceFile{targetSourceFile}
	} else {
		sourceFiles = host.SourceFiles()
	}
	return core.Filter(sourceFiles, func(sourceFile *ast.SourceFile) bool {
		return sourceFileMayBeEmitted(sourceFile, host, forceDtsEmit)
	})
}

func isSourceFileNotJson(file *ast.SourceFile) bool {
	return !ast.IsJsonSourceFile(file)
}

func getDeclarationDiagnostics(host EmitHost, file *ast.SourceFile) []*ast.Diagnostic {
	fullFiles := core.Filter(getSourceFilesToEmit(host, file, false), isSourceFileNotJson)
	if !core.Some(fullFiles, func(f *ast.SourceFile) bool { return f == file }) {
		return []*ast.Diagnostic{}
	}
	options := host.Options()
	transform := declarations.NewDeclarationTransformer(host, nil, options, "", "")
	transform.TransformSourceFile(file)
	return transform.GetDiagnostics()
}
