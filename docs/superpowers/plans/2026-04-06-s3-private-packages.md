# S3 Private Package Resolution Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow v3 endpoints to resolve private packages from S3 tarballs before running `bun install`, so callers can use packages not published to npm.

**Architecture:** Add a `resolve-s3` field to package.json. During `installDeps`, before `bun install`, download listed package tarballs from S3 and extract into node_modules. Bun sees them already installed and skips them. Requires changing `resolveDeps`/`installDeps` to accept parsed `*v3PackageJSON` and moving the `main` validation out of `parsePackageJSON`.

**Tech Stack:** Go, archive/tar, compress/gzip, AWS S3 SDK, regexp

**Spec:** `docs/superpowers/specs/2026-04-06-s3-private-packages-design.md`

---

## File Structure

| File | Change | Responsibility |
|------|--------|---------------|
| `server/v3_multipart.go` (modify) | Add `ResolveS3` field, remove `main` validation from `parsePackageJSON` |
| `server/v3_deps.go` (modify) | Change `resolveDeps`/`installDeps` signatures, add S3 pre-seeding logic |
| `server/v3_handlers.go` (modify) | Both handlers call `parsePackageJSON`, compile handler validates `main`, pass `pkg` to `resolveDeps` |
| `server/v3_test.go` (modify) | Tests for resolve-s3, version validation, tarball extraction, handler integration |

---

## Chunk 1: Struct and parsing changes

### Task 1: Add ResolveS3 to v3PackageJSON and move main validation

**Files:**
- Modify: `server/v3_multipart.go:106-111` (v3PackageJSON struct)
- Modify: `server/v3_multipart.go:214-223` (parsePackageJSON)
- Modify: `server/v3_test.go`

- [ ] **Step 1: Write failing tests**

Add to `server/v3_test.go`:

```go
func TestParsePackageJSON_ResolveS3(t *testing.T) {
	raw := []byte(`{
		"main": "./src/index.ts",
		"dependencies": {"@flickfyi/core": "0.0.8", "zod": "^3.23.0"},
		"resolve-s3": ["@flickfyi/core"]
	}`)
	pkg, err := parsePackageJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.ResolveS3) != 1 || pkg.ResolveS3[0] != "@flickfyi/core" {
		t.Fatalf("expected resolve-s3 [@flickfyi/core], got %v", pkg.ResolveS3)
	}
}

func TestParsePackageJSON_NoMainAllowed(t *testing.T) {
	// parsePackageJSON should no longer require main
	raw := []byte(`{"dependencies": {"zod": "^3.23.0"}}`)
	pkg, err := parsePackageJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Main != "" {
		t.Fatalf("expected empty main, got %s", pkg.Main)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd server && go test -run "TestParsePackageJSON_ResolveS3|TestParsePackageJSON_NoMainAllowed" -v`
Expected: `TestParsePackageJSON_NoMainAllowed` fails (parsePackageJSON currently requires main). `TestParsePackageJSON_ResolveS3` may pass if the field is ignored, but the struct doesn't have it yet.

- [ ] **Step 3: Implement changes**

In `server/v3_multipart.go`, add `ResolveS3` to the struct (alphabetical order):

```go
type v3PackageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Esbuild         v3EsbuildConfig   `json:"esbuild"`
	Main            string            `json:"main"`
	ResolveS3       []string          `json:"resolve-s3"`
}
```

Remove the `main` validation from `parsePackageJSON`:

```go
func parsePackageJSON(raw []byte) (*v3PackageJSON, error) {
	var pkg v3PackageJSON
	if err := json.Unmarshal(raw, &pkg); err != nil {
		return nil, fmt.Errorf("invalid package.json: %w", err)
	}
	return &pkg, nil
}
```

- [ ] **Step 4: Fix existing test that expects main validation**

The existing `TestParsePackageJSON_MissingMain` test expects an error when main is missing. This test should be updated — main is now validated in the compile handler, not in parsePackageJSON. Remove or update this test:

```go
func TestParsePackageJSON_MissingMain(t *testing.T) {
	// main is no longer required at parse time (only required for compile)
	raw := []byte(`{"dependencies": {"zod": "3.23.0"}}`)
	_, err := parsePackageJSON(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 5: Run all tests to verify**

Run: `cd server && go test -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add server/v3_multipart.go server/v3_test.go
git commit -m "Add resolve-s3 field to v3PackageJSON, move main validation to compile handler"
```

---

### Task 2: Update handlers to parse package.json in both paths and validate main in compile

**Files:**
- Modify: `server/v3_handlers.go`

- [ ] **Step 1: Write failing test**

Add to `server/v3_test.go`:

```go
func TestV3CompileHandler_MissingMain(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := []byte("missing-main-lock")
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash, "node_modules")
	os.MkdirAll(depDir, 0755)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{"dependencies": {}}`)
	writer.WriteField("/bun.lock", "missing-main-lock")
	writer.WriteField("/src/index.ts", "export const x = 1;")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v3/compile", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	compileV3Handler(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Result().StatusCode)
	}
}

func TestV3TypecheckHandler_NoMainRequired(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := []byte("no-main-typecheck-lock")
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash, "node_modules")
	os.MkdirAll(depDir, 0755)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{"dependencies": {}}`)
	writer.WriteField("/bun.lock", "no-main-typecheck-lock")
	writer.WriteField("/src/index.ts", "export const x: string = 'hello';")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	typecheckV3Handler(w, req)

	if w.Result().StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(w.Result().Body)
		t.Fatalf("expected 200, got %d: %s", w.Result().StatusCode, string(respBody))
	}
}
```

- [ ] **Step 2: Run tests to verify behavior**

Run: `cd server && go test -run "TestV3CompileHandler_MissingMain|TestV3TypecheckHandler_NoMainRequired" -v`
Expected: `TestV3TypecheckHandler_NoMainRequired` should pass now (parsePackageJSON no longer requires main, and typecheck handler doesn't call it). `TestV3CompileHandler_MissingMain` will fail — compile handler calls parsePackageJSON which no longer checks main.

- [ ] **Step 3: Update handlers**

Rewrite `server/v3_handlers.go`. Both handlers now call `parsePackageJSON`. The typecheck handler passes `pkg` to `resolveDeps`. The compile handler validates `main` after parsing. Change `resolveDeps` calls to pass `pkg` instead of raw bytes (this will require Task 3 to compile, so for now just update the handlers and let it fail to compile):

```go
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
	depPath, err := resolveDeps(context.Background(), lockContent, pkg)
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
	depPath, err := resolveDeps(context.Background(), lockContent, pkg)
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
```

Note: This won't compile yet — `resolveDeps` still takes `[]byte`. That's fixed in Task 3.

- [ ] **Step 4: Commit (will be combined with Task 3)**

Hold — commit after Task 3 when everything compiles.

---

### Task 3: Change resolveDeps/installDeps signatures and add S3 pre-seeding

**Files:**
- Modify: `server/v3_deps.go`

- [ ] **Step 1: Write failing tests**

Add to `server/v3_test.go`:

```go
func TestResolveS3Packages_VersionLookup(t *testing.T) {
	pkg := &v3PackageJSON{
		Dependencies: map[string]string{
			"@flickfyi/core": "0.0.8",
			"zod":            "^3.23.0",
		},
		ResolveS3: []string{"@flickfyi/core"},
	}

	version, err := lookupS3PackageVersion(pkg, "@flickfyi/core")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "0.0.8" {
		t.Fatalf("expected 0.0.8, got %s", version)
	}
}

func TestResolveS3Packages_NotInDeps(t *testing.T) {
	pkg := &v3PackageJSON{
		Dependencies: map[string]string{"zod": "^3.23.0"},
		ResolveS3:    []string{"@flickfyi/core"},
	}

	_, err := lookupS3PackageVersion(pkg, "@flickfyi/core")
	if err == nil {
		t.Fatal("expected error for package not in dependencies")
	}
}

func TestResolveS3Packages_RangeVersion(t *testing.T) {
	pkg := &v3PackageJSON{
		Dependencies: map[string]string{"@flickfyi/core": "^0.0.8"},
		ResolveS3:    []string{"@flickfyi/core"},
	}

	_, err := lookupS3PackageVersion(pkg, "@flickfyi/core")
	if err == nil {
		t.Fatal("expected error for non-exact version")
	}
}

func TestResolveS3Packages_DevDeps(t *testing.T) {
	pkg := &v3PackageJSON{
		DevDependencies: map[string]string{"@flickfyi/core": "0.0.8"},
		ResolveS3:       []string{"@flickfyi/core"},
	}

	version, err := lookupS3PackageVersion(pkg, "@flickfyi/core")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != "0.0.8" {
		t.Fatalf("expected 0.0.8, got %s", version)
	}
}

func TestExtractNpmTarball(t *testing.T) {
	// Create a tarball with package/ prefix (npm pack format)
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	pkgJSON := []byte(`{"name": "@flickfyi/core", "version": "0.0.8"}`)
	tw.WriteHeader(&tar.Header{Name: "package/package.json", Mode: 0644, Size: int64(len(pkgJSON))})
	tw.Write(pkgJSON)

	indexJS := []byte("exports.Flex = function() {};")
	tw.WriteHeader(&tar.Header{Name: "package/index.js", Mode: 0644, Size: int64(len(indexJS))})
	tw.Write(indexJS)

	tw.Close()
	gw.Close()

	tmpDir, err := os.MkdirTemp("", "tarball-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	destDir := filepath.Join(tmpDir, "node_modules", "@flickfyi", "core")

	err = extractNpmTarball(bytes.NewReader(buf.Bytes()), destDir)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	// Verify files extracted with package/ prefix stripped
	content, err := os.ReadFile(filepath.Join(destDir, "package.json"))
	if err != nil {
		t.Fatalf("package.json not found: %v", err)
	}
	if string(content) != `{"name": "@flickfyi/core", "version": "0.0.8"}` {
		t.Fatalf("unexpected content: %s", string(content))
	}

	content, err = os.ReadFile(filepath.Join(destDir, "index.js"))
	if err != nil {
		t.Fatalf("index.js not found: %v", err)
	}
	if string(content) != "exports.Flex = function() {};" {
		t.Fatalf("unexpected content: %s", string(content))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd server && go test -run "TestResolveS3Packages|TestExtractNpmTarball" -v`
Expected: Compile error — `lookupS3PackageVersion`, `extractNpmTarball` not defined

- [ ] **Step 3: Implement**

Add to `server/v3_deps.go`:

```go
import "regexp"

var exactSemverRe = regexp.MustCompile(`^\d+\.\d+\.\d+(-[\w.]+)?$`)

// lookupS3PackageVersion finds the version for a resolve-s3 package in dependencies or devDependencies.
// Returns an error if the package isn't listed or the version isn't exact semver.
func lookupS3PackageVersion(pkg *v3PackageJSON, name string) (string, error) {
	version := pkg.Dependencies[name]
	if version == "" {
		version = pkg.DevDependencies[name]
	}
	if version == "" {
		return "", fmt.Errorf("resolve-s3 package %q not found in dependencies or devDependencies", name)
	}
	if !exactSemverRe.MatchString(version) {
		return "", fmt.Errorf("resolve-s3 package %q must use an exact version, got %q", name, version)
	}
	return version, nil
}

// extractNpmTarball extracts an npm tarball (gzipped tar with package/ prefix) into destDir,
// stripping the package/ prefix.
func extractNpmTarball(r io.Reader, destDir string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar next: %w", err)
		}

		// Strip "package/" prefix
		name := header.Name
		if strings.HasPrefix(name, "package/") {
			name = strings.TrimPrefix(name, "package/")
		}
		if name == "" || name == "." {
			continue
		}

		// Sanitize
		cleanName := filepath.Clean(name)
		if filepath.IsAbs(cleanName) {
			continue
		}

		target := filepath.Join(destDir, cleanName)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}

// preseedS3Packages downloads and extracts resolve-s3 packages into node_modules.
func preseedS3Packages(ctx context.Context, pkg *v3PackageJSON, nodeModulesDir string) error {
	for _, name := range pkg.ResolveS3 {
		version, err := lookupS3PackageVersion(pkg, name)
		if err != nil {
			return err
		}

		key := "packages/" + name + "/" + version + ".tgz"
		out, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s3Bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return fmt.Errorf("failed to download %s@%s from S3 (%s): %w", name, version, key, err)
		}

		destDir := filepath.Join(nodeModulesDir, name)
		if err := extractNpmTarball(out.Body, destDir); err != nil {
			out.Body.Close()
			return fmt.Errorf("failed to extract %s@%s: %w", name, version, err)
		}
		out.Body.Close()

		log.Printf("[DEPS] Pre-seeded %s@%s from S3", name, version)
	}
	return nil
}
```

Now change `resolveDeps` signature from `(ctx, lockContent []byte, packageJSON []byte)` to `(ctx, lockContent []byte, pkg *v3PackageJSON, rawPackageJSON []byte)`:

In the `resolveDeps` function, replace the `packageJSON []byte` parameter with `pkg *v3PackageJSON, rawPackageJSON []byte`. Pass both through to `installDeps`.

Change `installDeps` signature from `(ctx, hash, depDir string, lockContent []byte, packageJSON []byte)` to `(ctx, hash, depDir string, lockContent []byte, pkg *v3PackageJSON, rawPackageJSON []byte)`.

In `installDeps`, keep writing `rawPackageJSON` to disk (not re-marshaling, which would lose fields like `name`, `version`, `type`, etc. that bun may need). After writing files and before `bun install`, add the preseed step:

```go
// Write the ORIGINAL package.json (preserves all fields bun needs)
if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), rawPackageJSON, 0644); err != nil {
	return "", fmt.Errorf("write package.json: %w", err)
}

// ... write bun.lock ...

// Pre-seed S3 packages before bun install
if len(pkg.ResolveS3) > 0 {
	nmDir := filepath.Join(tmpDir, "node_modules")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		return "", fmt.Errorf("mkdir node_modules: %w", err)
	}
	if err := preseedS3Packages(ctx, pkg, nmDir); err != nil {
		return "", fmt.Errorf("preseed s3 packages: %w", err)
	}
}

// bun install ...
```

**Update existing test callers**: The tests `TestResolveDeps_LocalHit` and `TestResolveDeps_S3Hit` call `resolveDeps` directly with `[]byte`. Update them to pass a `*v3PackageJSON` and raw bytes:

```go
// In TestResolveDeps_LocalHit and TestResolveDeps_S3Hit, change:
//   resolveDeps(context.Background(), lockContent, []byte(`{"dependencies":{"zod":"3.23.0"}}`))
// To:
pkg := &v3PackageJSON{Dependencies: map[string]string{"zod": "3.23.0"}}
rawPkg := []byte(`{"dependencies":{"zod":"3.23.0"}}`)
resolveDeps(context.Background(), lockContent, pkg, rawPkg)
```

**Update handler calls**: In both handlers, change `resolveDeps(ctx, lockContent, files["/package.json"])` to `resolveDeps(ctx, lockContent, pkg, files["/package.json"])`.

Also add `"regexp"` and `"strings"` to the v3_deps.go imports.

- [ ] **Step 4: Run all tests**

Run: `cd server && go test -v`
Expected: All PASS (existing tests use `resolveDeps` via handlers which now pass `pkg`)

- [ ] **Step 5: Commit**

```bash
git add server/v3_deps.go server/v3_handlers.go server/v3_multipart.go server/v3_test.go
git commit -m "Add resolve-s3 private package pre-seeding from S3 tarballs"
```

---

## Chunk 2: Integration tests

### Task 4: Integration tests for resolve-s3

**Files:**
- Modify: `server/v3_test.go`

- [ ] **Step 1: Write integration tests**

Add to `server/v3_test.go`:

```go
func TestV3TypecheckHandler_WithS3Packages(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := []byte("s3-pkg-typecheck-lock")
	hash := hashBunLock(lockContent)

	// Pre-populate dep cache with the S3 package already extracted
	// (simulates a cache hit — we don't actually run bun install in tests)
	depBase := filepath.Join(diskCachePath, "deps", hash)
	nmDir := filepath.Join(depBase, "node_modules")
	coreDir := filepath.Join(nmDir, "@flickfyi", "core")
	os.MkdirAll(coreDir, 0755)
	os.WriteFile(filepath.Join(coreDir, "package.json"), []byte(`{"name": "@flickfyi/core", "version": "0.0.8", "main": "index.js", "types": "index.d.ts"}`), 0644)
	os.WriteFile(filepath.Join(coreDir, "index.d.ts"), []byte(`export declare function Flex(props: any): any;`), 0644)
	os.WriteFile(filepath.Join(coreDir, "index.js"), []byte(`exports.Flex = function() {};`), 0644)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{
		"dependencies": {"@flickfyi/core": "0.0.8"},
		"resolve-s3": ["@flickfyi/core"]
	}`)
	writer.WriteField("/bun.lock", "s3-pkg-typecheck-lock")
	writer.WriteField("/src/index.ts", "import { Flex } from '@flickfyi/core';\nexport const f = Flex;")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	typecheckV3Handler(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	var result TypecheckV2Response
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Pass {
		t.Fatalf("expected pass, got errors: %v", result.Errors)
	}
}

func TestV3_ResolveS3_BadVersion(t *testing.T) {
	setupTestServerWithMockS3(t)

	// No disk cache → falls through to tier 3 (install) → preseed fails on version validation
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{
		"dependencies": {"@flickfyi/core": "^0.0.8"},
		"resolve-s3": ["@flickfyi/core"]
	}`)
	writer.WriteField("/bun.lock", "bad-version-lock")
	writer.WriteField("/src/index.ts", "export const x = 1;")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	typecheckV3Handler(w, req)

	// preseedS3Packages returns error before bun install runs → handler returns 502
	if w.Result().StatusCode != http.StatusBadGateway {
		respBody, _ := io.ReadAll(w.Result().Body)
		t.Fatalf("expected 502, got %d: %s", w.Result().StatusCode, string(respBody))
	}
}

func TestV3_ResolveS3_NotInDeps(t *testing.T) {
	setupTestServerWithMockS3(t)

	// No disk cache → falls through to tier 3 → preseed fails on missing dep
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{
		"dependencies": {"zod": "^3.23.0"},
		"resolve-s3": ["@flickfyi/core"]
	}`)
	writer.WriteField("/bun.lock", "not-in-deps-lock")
	writer.WriteField("/src/index.ts", "export const x = 1;")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	typecheckV3Handler(w, req)

	// preseedS3Packages returns error before bun install runs → handler returns 502
	if w.Result().StatusCode != http.StatusBadGateway {
		respBody, _ := io.ReadAll(w.Result().Body)
		t.Fatalf("expected 502, got %d: %s", w.Result().StatusCode, string(respBody))
	}
}

func TestV3_ResolveS3_EmptyList(t *testing.T) {
	setupTestServerWithMockS3(t)

	lockContent := []byte("empty-resolve-s3-lock")
	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash, "node_modules")
	os.MkdirAll(depDir, 0755)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("/package.json", `{"dependencies": {}, "resolve-s3": []}`)
	writer.WriteField("/bun.lock", "empty-resolve-s3-lock")
	writer.WriteField("/src/index.ts", "export const x: string = 'hello';")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v3/typecheck", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	typecheckV3Handler(w, req)

	if w.Result().StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(w.Result().Body)
		t.Fatalf("expected 200, got %d: %s", w.Result().StatusCode, string(respBody))
	}
}
```

- [ ] **Step 2: Run all tests**

Run: `cd server && go test -v`
Expected: All PASS

- [ ] **Step 3: Commit**

```bash
git add server/v3_test.go
git commit -m "Add integration tests for resolve-s3 private package resolution"
```
