package incremental

import (
	"context"
	"maps"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/collections"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tspath"
)

type dtsMayChange map[tspath.Path]FileEmitKind

func (c dtsMayChange) addFileToAffectedFilesPendingEmit(filePath tspath.Path, emitKind FileEmitKind) {
	c[filePath] = emitKind
}

type affectedFilesHandler struct {
	ctx                                    context.Context
	program                                *Program
	hasAllFilesExcludingDefaultLibraryFile atomic.Bool
	updatedSignatures                      collections.SyncMap[tspath.Path, string]
	updatedSignatureKinds                  *collections.SyncMap[tspath.Path, SignatureUpdateKind]
	dtsMayChange                           []dtsMayChange
	filesToRemoveDiagnostics               collections.SyncSet[tspath.Path]
	cleanedDiagnosticsOfLibFiles           sync.Once
	seenFileAndExportsOfFile               collections.SyncMap[tspath.Path, bool]
}

func (h *affectedFilesHandler) getDtsMayChange(affectedFilePath tspath.Path, affectedFileEmitKind FileEmitKind) dtsMayChange {
	result := dtsMayChange(map[tspath.Path]FileEmitKind{affectedFilePath: affectedFileEmitKind})
	h.dtsMayChange = append(h.dtsMayChange, result)
	return result
}

func (h *affectedFilesHandler) isChangedSignature(path tspath.Path) bool {
	newSignature, _ := h.updatedSignatures.Load(path)
	oldSignature := h.program.snapshot.fileInfos[path].signature
	return newSignature != oldSignature
}

func (h *affectedFilesHandler) removeSemanticDiagnosticsOf(path tspath.Path) {
	h.filesToRemoveDiagnostics.Add(path)
}

func (h *affectedFilesHandler) removeDiagnosticsOfLibraryFiles() {
	h.cleanedDiagnosticsOfLibFiles.Do(func() {
		for _, file := range h.program.GetSourceFiles() {
			if h.program.program.IsSourceFileDefaultLibrary(file.Path()) && !checker.SkipTypeChecking(file, h.program.snapshot.options, h.program.program, true) {
				h.removeSemanticDiagnosticsOf(file.Path())
			}
		}
	})
}

func (h *affectedFilesHandler) computeDtsSignature(file *ast.SourceFile) string {
	var signature string
	h.program.program.Emit(h.ctx, compiler.EmitOptions{
		TargetSourceFile: file,
		EmitOnly:         compiler.EmitOnlyForcedDts,
		WriteFile: func(fileName string, text string, writeByteOrderMark bool, data *compiler.WriteFileData) error {
			if !tspath.IsDeclarationFileName(fileName) {
				panic("File extension for signature expected to be dts, got : " + fileName)
			}
			signature = h.program.snapshot.computeSignatureWithDiagnostics(file, text, data)
			return nil
		},
	})
	return signature
}

func (h *affectedFilesHandler) updateShapeSignature(file *ast.SourceFile, useFileVersionAsSignature bool) bool {
	// If we have cached the result for this file, that means hence forth we should assume file shape is uptodate
	if _, ok := h.updatedSignatures.Load(file.Path()); ok {
		return false
	}

	info := h.program.snapshot.fileInfos[file.Path()]
	prevSignature := info.signature
	var latestSignature string
	var updateKind SignatureUpdateKind
	if !file.IsDeclarationFile && !useFileVersionAsSignature {
		latestSignature = h.computeDtsSignature(file)
	}
	// Default is to use file version as signature
	if latestSignature == "" {
		latestSignature = info.version
		updateKind = SignatureUpdateKindUsedVersion
	}
	h.updatedSignatures.Store(file.Path(), latestSignature)
	if h.updatedSignatureKinds != nil {
		h.updatedSignatureKinds.Store(file.Path(), updateKind)
	}
	return latestSignature != prevSignature
}

func (h *affectedFilesHandler) getFilesAffectedBy(path tspath.Path) []*ast.SourceFile {
	file := h.program.program.GetSourceFileByPath(path)
	if file == nil {
		return nil
	}

	if !h.updateShapeSignature(file, false) {
		return []*ast.SourceFile{file}
	}

	if info := h.program.snapshot.fileInfos[file.Path()]; info.affectsGlobalScope {
		h.hasAllFilesExcludingDefaultLibraryFile.Store(true)
		h.program.snapshot.getAllFilesExcludingDefaultLibraryFile(h.program.program, file)
	}

	if h.program.snapshot.options.IsolatedModules.IsTrue() {
		return []*ast.SourceFile{file}
	}

	// Now we need to if each file in the referencedBy list has a shape change as well.
	// Because if so, its own referencedBy files need to be saved as well to make the
	// emitting result consistent with files on disk.
	seenFileNamesMap := h.forEachFileReferencedBy(
		file,
		func(currentFile *ast.SourceFile, currentPath tspath.Path) (queueForFile bool, fastReturn bool) {
			// If the current file is not nil and has a shape change, we need to queue it for processing
			if currentFile != nil && h.updateShapeSignature(currentFile, false) {
				return true, false
			}
			return false, false
		},
	)
	// Return array of values that needs emit
	return core.Filter(slices.Collect(maps.Values(seenFileNamesMap)), func(file *ast.SourceFile) bool {
		return file != nil
	})
}

// Gets the files referenced by the the file path
func (h *affectedFilesHandler) getReferencedByPaths(file tspath.Path) map[tspath.Path]struct{} {
	keys, ok := h.program.snapshot.referencedMap.GetKeys(file)
	if !ok {
		return nil
	}
	return keys.Keys()
}

func (h *affectedFilesHandler) forEachFileReferencedBy(file *ast.SourceFile, fn func(currentFile *ast.SourceFile, currentPath tspath.Path) (queueForFile bool, fastReturn bool)) map[tspath.Path]*ast.SourceFile {
	// Now we need to if each file in the referencedBy list has a shape change as well.
	// Because if so, its own referencedBy files need to be saved as well to make the
	// emitting result consistent with files on disk.
	seenFileNamesMap := map[tspath.Path]*ast.SourceFile{}
	// Start with the paths this file was referenced by
	seenFileNamesMap[file.Path()] = file
	references := h.getReferencedByPaths(file.Path())
	queue := slices.Collect(maps.Keys(references))
	for len(queue) > 0 {
		currentPath := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if _, ok := seenFileNamesMap[currentPath]; !ok {
			currentFile := h.program.program.GetSourceFileByPath(currentPath)
			seenFileNamesMap[currentPath] = currentFile
			queueForFile, fastReturn := fn(currentFile, currentPath)
			if fastReturn {
				return seenFileNamesMap
			}
			if queueForFile {
				for ref := range h.getReferencedByPaths(currentFile.Path()) {
					queue = append(queue, ref)
				}
			}
		}
	}
	return seenFileNamesMap
}

// Handles semantic diagnostics and dts emit for affectedFile and files, that are referencing modules that export entities from affected file
// This is because even though js emit doesnt change, dts emit / type used can change resulting in need for dts emit and js change
func (h *affectedFilesHandler) handleDtsMayChangeOfAffectedFile(dtsMayChange dtsMayChange, affectedFile *ast.SourceFile) {
	h.removeSemanticDiagnosticsOf(affectedFile.Path())

	// If affected files is everything except default library, then nothing more to do
	if h.hasAllFilesExcludingDefaultLibraryFile.Load() {
		h.removeDiagnosticsOfLibraryFiles()
		// When a change affects the global scope, all files are considered to be affected without updating their signature
		// That means when affected file is handled, its signature can be out of date
		// To avoid this, ensure that we update the signature for any affected file in this scenario.
		h.updateShapeSignature(affectedFile, false)
		return
	}

	if h.program.snapshot.options.AssumeChangesOnlyAffectDirectDependencies.IsTrue() {
		return
	}

	// Iterate on referencing modules that export entities from affected file and delete diagnostics and add pending emit
	// If there was change in signature (dts output) for the changed file,
	// then only we need to handle pending file emit
	if !h.program.snapshot.changedFilesSet.Has(affectedFile.Path()) ||
		!h.isChangedSignature(affectedFile.Path()) {
		return
	}

	// Since isolated modules dont change js files, files affected by change in signature is itself
	// But we need to cleanup semantic diagnostics and queue dts emit for affected files
	if h.program.snapshot.options.IsolatedModules.IsTrue() {
		h.forEachFileReferencedBy(
			affectedFile,
			func(currentFile *ast.SourceFile, currentPath tspath.Path) (queueForFile bool, fastReturn bool) {
				if h.handleDtsMayChangeOfGlobalScope(dtsMayChange, currentPath /*invalidateJsFiles*/, false) {
					return false, true
				}
				h.handleDtsMayChangeOf(dtsMayChange, currentPath /*invalidateJsFiles*/, false)
				if h.isChangedSignature(currentPath) {
					return true, false
				}
				return false, false
			},
		)
	}

	invalidateJsFiles := false
	var typeChecker *checker.Checker
	var done func()
	// If exported const enum, we need to ensure that js files are emitted as well since the const enum value changed
	if affectedFile.Symbol != nil {
		for _, exported := range affectedFile.Symbol.Exports {
			if exported.Flags&ast.SymbolFlagsConstEnum != 0 {
				invalidateJsFiles = true
				break
			}
			if typeChecker == nil {
				typeChecker, done = h.program.program.GetTypeCheckerForFile(h.ctx, affectedFile)
			}
			aliased := checker.SkipAlias(exported, typeChecker)
			if aliased == exported {
				continue
			}
			if (aliased.Flags & ast.SymbolFlagsConstEnum) != 0 {
				if slices.ContainsFunc(aliased.Declarations, func(d *ast.Node) bool {
					return ast.GetSourceFileOfNode(d) == affectedFile
				}) {
					invalidateJsFiles = true
					break
				}
			}
		}
	}
	if done != nil {
		done()
	}

	// Go through files that reference affected file and handle dts emit and semantic diagnostics for them and their references
	if keys, ok := h.program.snapshot.referencedMap.GetKeys(affectedFile.Path()); ok {
		for exportedFromPath := range keys.Keys() {
			if h.handleDtsMayChangeOfGlobalScope(dtsMayChange, exportedFromPath, invalidateJsFiles) {
				return
			}
			if references, ok := h.program.snapshot.referencedMap.GetKeys(exportedFromPath); ok {
				for filePath := range references.Keys() {
					if h.handleDtsMayChangeOfFileAndExportsOfFile(dtsMayChange, filePath, invalidateJsFiles) {
						return
					}
				}
			}
		}
	}
}

func (h *affectedFilesHandler) handleDtsMayChangeOfFileAndExportsOfFile(dtsMayChange dtsMayChange, filePath tspath.Path, invalidateJsFiles bool) bool {
	if existing, loaded := h.seenFileAndExportsOfFile.LoadOrStore(filePath, invalidateJsFiles); loaded && (existing || !invalidateJsFiles) {
		return false
	}
	if h.handleDtsMayChangeOfGlobalScope(dtsMayChange, filePath, invalidateJsFiles) {
		return true
	}
	h.handleDtsMayChangeOf(dtsMayChange, filePath, invalidateJsFiles)

	// Remove the diagnostics of files that import this file and handle all its exports too
	if keys, ok := h.program.snapshot.referencedMap.GetKeys(filePath); ok {
		for referencingFilePath := range keys.Keys() {
			if h.handleDtsMayChangeOfFileAndExportsOfFile(dtsMayChange, referencingFilePath, invalidateJsFiles) {
				return true
			}
		}
	}
	return false
}

func (h *affectedFilesHandler) handleDtsMayChangeOfGlobalScope(dtsMayChange dtsMayChange, filePath tspath.Path, invalidateJsFiles bool) bool {
	if info, ok := h.program.snapshot.fileInfos[filePath]; !ok || !info.affectsGlobalScope {
		return false
	}
	// Every file needs to be handled
	for _, file := range h.program.snapshot.getAllFilesExcludingDefaultLibraryFile(h.program.program, nil) {
		h.handleDtsMayChangeOf(dtsMayChange, file.Path(), invalidateJsFiles)
	}
	h.removeDiagnosticsOfLibraryFiles()
	return true
}

// Handle the dts may change, so they need to be added to pending emit if dts emit is enabled,
// Also we need to make sure signature is updated for these files
func (h *affectedFilesHandler) handleDtsMayChangeOf(dtsMayChange dtsMayChange, path tspath.Path, invalidateJsFiles bool) {
	if h.program.snapshot.changedFilesSet.Has(path) {
		return
	}
	file := h.program.program.GetSourceFileByPath(path)
	if file == nil {
		return
	}
	h.removeSemanticDiagnosticsOf(path)
	// Even though the js emit doesnt change and we are already handling dts emit and semantic diagnostics
	// we need to update the signature to reflect correctness of the signature(which is output d.ts emit) of this file
	// This ensures that we dont later during incremental builds considering wrong signature.
	// Eg where this also is needed to ensure that .tsbuildinfo generated by incremental build should be same as if it was first fresh build
	// But we avoid expensive full shape computation, as using file version as shape is enough for correctness.
	h.updateShapeSignature(file, true)
	// If not dts emit, nothing more to do
	if invalidateJsFiles {
		dtsMayChange.addFileToAffectedFilesPendingEmit(path, GetFileEmitKind(h.program.snapshot.options))
	} else if h.program.snapshot.options.GetEmitDeclarations() {
		dtsMayChange.addFileToAffectedFilesPendingEmit(path, core.IfElse(h.program.snapshot.options.DeclarationMap.IsTrue(), FileEmitKindAllDts, FileEmitKindDts))
	}
}

func (h *affectedFilesHandler) updateSnapshot() {
	if h.ctx.Err() != nil {
		return
	}
	h.updatedSignatures.Range(func(filePath tspath.Path, signature string) bool {
		h.program.snapshot.fileInfos[filePath].signature = signature
		return true
	})
	if h.updatedSignatureKinds != nil {
		h.updatedSignatureKinds.Range(func(filePath tspath.Path, kind SignatureUpdateKind) bool {
			h.program.updatedSignatureKinds[filePath] = kind
			return true
		})
	}
	h.filesToRemoveDiagnostics.Range(func(file tspath.Path) bool {
		delete(h.program.snapshot.semanticDiagnosticsPerFile, file)
		return true
	})
	for _, change := range h.dtsMayChange {
		for filePath, emitKind := range change {
			h.program.snapshot.addFileToAffectedFilesPendingEmit(filePath, emitKind)
		}
	}
	h.program.snapshot.changedFilesSet = &collections.Set[tspath.Path]{}
	h.program.snapshot.buildInfoEmitPending = true
}

func collectAllAffectedFiles(ctx context.Context, program *Program) {
	if program.snapshot.changedFilesSet.Len() == 0 {
		return
	}

	handler := affectedFilesHandler{ctx: ctx, program: program, updatedSignatureKinds: core.IfElse(program.updatedSignatureKinds == nil, nil, &collections.SyncMap[tspath.Path, SignatureUpdateKind]{})}
	wg := core.NewWorkGroup(handler.program.program.SingleThreaded())
	var result collections.SyncSet[*ast.SourceFile]
	for file := range program.snapshot.changedFilesSet.Keys() {
		wg.Queue(func() {
			for _, affectedFile := range handler.getFilesAffectedBy(file) {
				result.Add(affectedFile)
			}
		})
	}
	wg.RunAndWait()

	if ctx.Err() != nil {
		return
	}

	// For all the affected files, get all the files that would need to change their dts or js files,
	// update their diagnostics
	wg = core.NewWorkGroup(program.program.SingleThreaded())
	emitKind := GetFileEmitKind(program.snapshot.options)
	result.Range(func(file *ast.SourceFile) bool {
		// remove the cached semantic diagnostics and handle dts emit and js emit if needed
		dtsMayChange := handler.getDtsMayChange(file.Path(), emitKind)
		wg.Queue(func() {
			handler.handleDtsMayChangeOfAffectedFile(dtsMayChange, file)
		})
		return true
	})
	wg.RunAndWait()

	// Update the snapshot with the new state
	handler.updateSnapshot()
}
