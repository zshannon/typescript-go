package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
)

func typecheckV3Handler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := req.Context()
	_, decodeSpan := startSpan(ctx, "http.request.body.decode")
	files, err := parseV3Multipart(req.Body, req.Header.Get("Content-Type"))
	recordSpanError(decodeSpan, "err-multipart-decode", err)
	decodeSpan.End()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, packageSpan := startSpan(ctx, "package_json.decode")
	pkg, err := parsePackageJSON(files["/package.json"])
	recordSpanError(packageSpan, "err-package-json-decode", err)
	packageSpan.End()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	lockContent := files["/bun.lock"]
	depPath, releaseDeps, err := resolveDepsForUse(depResolveContext(ctx), lockContent, pkg, files["/package.json"])
	if err != nil {
		log.Printf("[V3] Dep resolution failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		writeJSONResponse(ctx, w, TypecheckV2Response{
			Errors: []DiagnosticErrorV2{{Message: "dependency installation failed: " + err.Error()}},
		})
		return
	}
	defer releaseDeps()
	_ = depPath

	var tsconfigRaw []byte
	if tc, ok := files["/tsconfig.json"]; ok {
		tsconfigRaw = tc
	}

	response := typecheckV3WithContext(ctx, files, tsconfigRaw, lockContent)

	w.Header().Set("Content-Type", "application/json")
	writeJSONResponse(ctx, w, response)
}

func compileV3Handler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := req.Context()
	_, decodeSpan := startSpan(ctx, "http.request.body.decode")
	files, err := parseV3Multipart(req.Body, req.Header.Get("Content-Type"))
	recordSpanError(decodeSpan, "err-multipart-decode", err)
	decodeSpan.End()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, packageSpan := startSpan(ctx, "package_json.decode")
	pkg, err := parsePackageJSON(files["/package.json"])
	recordSpanError(packageSpan, "err-package-json-decode", err)
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
	depPath, releaseDeps, err := resolveDepsForUse(depResolveContext(ctx), lockContent, pkg, files["/package.json"])
	if err != nil {
		log.Printf("[V3] Dep resolution failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		writeJSONResponse(ctx, w, BuildV2Response{
			Errors: []DiagnosticErrorV2{{Message: "dependency installation failed: " + err.Error()}},
		})
		return
	}
	defer releaseDeps()
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
			writeJSONResponse(ctx, w, BuildV2Response{Errors: typecheckResponse.Errors})
			return
		}
	}

	response := compileV3WithContext(ctx, files, pkg, tsconfigRaw, lockContent)

	w.Header().Set("Content-Type", "application/json")
	writeJSONResponse(ctx, w, response)
}

func writeJSONResponse(ctx context.Context, w http.ResponseWriter, response any) {
	_, span := startSpan(ctx, "http.response.body.encode")
	defer span.End()
	recordSpanError(span, "err-json-response-encode", json.NewEncoder(w).Encode(response))
}
