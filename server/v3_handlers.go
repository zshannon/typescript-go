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

	files, err := parseV3Multipart(req.Body, req.Header.Get("Content-Type"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pkg, err := parsePackageJSON(files["/package.json"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	lockContent := files["/bun.lock"]
	depPath, err := resolveDeps(context.Background(), lockContent, pkg, files["/package.json"])
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

	response := typecheckV3(files, tsconfigRaw, lockContent)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func compileV3Handler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	files, err := parseV3Multipart(req.Body, req.Header.Get("Content-Type"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pkg, err := parsePackageJSON(files["/package.json"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if pkg.Main == "" {
		http.Error(w, "package.json missing required field: main", http.StatusBadRequest)
		return
	}

	lockContent := files["/bun.lock"]
	depPath, err := resolveDeps(context.Background(), lockContent, pkg, files["/package.json"])
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
		typecheckResponse := typecheckV3(files, tsconfigRaw, lockContent)
		if len(typecheckResponse.Errors) > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(BuildV2Response{Errors: typecheckResponse.Errors})
			return
		}
	}

	response := compileV3(files, pkg, tsconfigRaw, lockContent)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
