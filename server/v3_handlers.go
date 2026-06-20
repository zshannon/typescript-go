package main

import (
	"encoding/json"
	"log"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
)

func typecheckV3Handler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := req.Context()
	_, parseSpan := startSpan(ctx, "fly_tsgo.v3.multipart.parse")
	files, err := parseV3Multipart(req.Body, req.Header.Get("Content-Type"))
	if err == nil {
		parseSpan.SetAttributes(v3FileAttributes(files)...)
	} else {
		recordSpanError(parseSpan, "err-v3-typecheck-parse-multipart", err)
	}
	parseSpan.End()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, packageSpan := startSpan(ctx, "fly_tsgo.v3.package_json.parse")
	pkg, err := parsePackageJSON(files["/package.json"])
	recordSpanError(packageSpan, "err-v3-typecheck-parse-package-json", err)
	packageSpan.End()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	lockContent := files["/bun.lock"]
	depPath, err := resolveDeps(ctx, lockContent, pkg, files["/package.json"])
	if err != nil {
		log.Printf("[V3] Dep resolution failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(TypecheckV2Response{
			Errors: []DiagnosticErrorV2{{Message: "dependency installation failed: " + err.Error()}},
		})
		return
	}
	_ = depPath

	var tsconfigRaw []byte
	if tc, ok := files["/tsconfig.json"]; ok {
		tsconfigRaw = tc
	}

	response := typecheckV3WithContext(ctx, files, tsconfigRaw, lockContent)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func compileV3Handler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := req.Context()
	_, parseSpan := startSpan(ctx, "fly_tsgo.v3.multipart.parse")
	files, err := parseV3Multipart(req.Body, req.Header.Get("Content-Type"))
	if err == nil {
		parseSpan.SetAttributes(v3FileAttributes(files)...)
	} else {
		recordSpanError(parseSpan, "err-v3-compile-parse-multipart", err)
	}
	parseSpan.End()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, packageSpan := startSpan(ctx, "fly_tsgo.v3.package_json.parse")
	pkg, err := parsePackageJSON(files["/package.json"])
	recordSpanError(packageSpan, "err-v3-compile-parse-package-json", err)
	packageSpan.End()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if pkg.Main == "" {
		http.Error(w, "package.json missing required field: main", http.StatusBadRequest)
		return
	}

	lockContent := files["/bun.lock"]
	depPath, err := resolveDeps(ctx, lockContent, pkg, files["/package.json"])
	if err != nil {
		log.Printf("[V3] Dep resolution failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(BuildV2Response{
			Errors: []DiagnosticErrorV2{{Message: "dependency installation failed: " + err.Error()}},
		})
		return
	}
	_ = depPath

	var tsconfigRaw []byte
	if tc, ok := files["/tsconfig.json"]; ok {
		tsconfigRaw = tc
	}

	skipTypecheck := req.URL.Query().Get("skip_typecheck") == "true"
	if !skipTypecheck {
		typecheckResponse := typecheckV3WithContext(ctx, files, tsconfigRaw, lockContent)
		if len(typecheckResponse.Errors) > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildV2Response{Errors: typecheckResponse.Errors})
			return
		}
	}

	response := compileV3WithContext(ctx, files, pkg, tsconfigRaw, lockContent)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func v3FileAttributes(files map[string][]byte) []attribute.KeyValue {
	var totalBytes int
	for _, content := range files {
		totalBytes += len(content)
	}
	return []attribute.KeyValue{
		attribute.Int("fly_tsgo.files.count", len(files)),
		attribute.Int("fly_tsgo.files.total_bytes", totalBytes),
	}
}
