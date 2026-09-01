package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/attribute"
)

const (
	activationOverridePath = "/.flick/activation.json"
	hostContractPrefix     = "/.flick/host/"
)

var hostContractFileNames = []string{
	"index.d.ts",
	"index.js",
	"manifest.json",
	"package.json",
}

var hostPackageNamePattern = regexp.MustCompile(`^[a-z0-9.-]+$`)

type activationOverride struct {
	Scope string `json:"scope"`
}

type hostCompilationContext struct {
	contract      *hostContract
	overrideScope string
}

type hostContract struct {
	declarations []byte
	manifest     hostContractManifest
	manifestRaw  []byte
	packageJSON  hostContractPackage
	packageRaw   []byte
	runtime      []byte
}

type hostContractManifest struct {
	Exports map[string]hostContractExport `json:"exports"`
	Name    string                        `json:"name"`
	Runtime string                        `json:"runtime"`
	Types   string                        `json:"types"`
}

type hostContractExport struct {
	Types string `json:"types"`
}

type hostContractPackage struct {
	Exports          map[string]hostContractPackageExport `json:"exports"`
	Name             string                               `json:"name"`
	PeerDependencies map[string]string                    `json:"peerDependencies"`
	Private          bool                                 `json:"private"`
	Type             string                               `json:"type"`
	Types            string                               `json:"types"`
}

type hostContractPackageExport struct {
	Default string `json:"default"`
	Types   string `json:"types"`
}

func parseHostCompilationContext(files map[string][]byte) (*hostCompilationContext, error) {
	context := &hostCompilationContext{}
	contract, err := parseHostContract(files)
	if err != nil {
		return nil, err
	}
	context.contract = contract

	if raw, ok := files[activationOverridePath]; ok {
		if contract == nil {
			return nil, fmt.Errorf("%s requires a host contract", activationOverridePath)
		}
		var override activationOverride
		if err := decodeStrictJSON(raw, &override); err != nil {
			return nil, fmt.Errorf("invalid %s: %w", activationOverridePath, err)
		}
		if override.Scope == "" {
			return nil, fmt.Errorf("%s missing required field: scope", activationOverridePath)
		}
		if !contract.hasScope(override.Scope) {
			return nil, fmt.Errorf("activation override scope is not exported by %s: %s", contract.manifest.Name, override.Scope)
		}
		context.overrideScope = override.Scope
	}

	return context, nil
}

func parseHostCompilationContextWithTracing(ctx context.Context, files map[string][]byte) (result *hostCompilationContext, err error) {
	hostFileCount := 0
	for fileName := range files {
		if strings.HasPrefix(fileName, hostContractPrefix) {
			hostFileCount++
		}
	}
	_, overridePresent := files[activationOverridePath]
	_, span := startSpan(ctx, "fly_tsgo.host_contract.parse",
		attribute.Int("fly_tsgo.host_contract.files.count", hostFileCount),
		attribute.Bool("fly_tsgo.host_contract.override.present", overridePresent),
		attribute.Bool("fly_tsgo.host_contract.provided", hostFileCount > 0),
	)
	defer func() {
		span.SetAttributes(attribute.Bool("fly_tsgo.host_contract.success", err == nil))
		if result != nil && result.contract != nil {
			span.SetAttributes(
				attribute.Int("fly_tsgo.host_contract.declarations.bytes", len(result.contract.declarations)),
				attribute.Int("fly_tsgo.host_contract.exports.count", len(result.contract.manifest.Exports)),
				attribute.String("fly_tsgo.host_contract.provider", result.contract.manifest.Name),
				attribute.Int("fly_tsgo.host_contract.runtime.bytes", len(result.contract.runtime)),
			)
		}
		recordSpanError(span, "err-host-contract-parse", err)
		span.End()
	}()

	return parseHostCompilationContext(files)
}

func parseHostContract(files map[string][]byte) (*hostContract, error) {
	found := 0
	for fileName := range files {
		if !strings.HasPrefix(fileName, hostContractPrefix) {
			continue
		}
		if !isHostContractFile(fileName) {
			return nil, fmt.Errorf("unexpected host contract file: %s", fileName)
		}
		found++
	}
	if found == 0 {
		return nil, nil
	}
	if found != len(hostContractFileNames) {
		return nil, fmt.Errorf("host contract requires index.js, index.d.ts, manifest.json, and package.json")
	}

	contract := &hostContract{
		declarations: files[hostContractPrefix+"index.d.ts"],
		manifestRaw:  files[hostContractPrefix+"manifest.json"],
		packageRaw:   files[hostContractPrefix+"package.json"],
		runtime:      files[hostContractPrefix+"index.js"],
	}
	if len(contract.declarations) == 0 || len(contract.runtime) == 0 {
		return nil, fmt.Errorf("host contract runtime and declarations must not be empty")
	}
	if err := decodeStrictJSON(contract.manifestRaw, &contract.manifest); err != nil {
		return nil, fmt.Errorf("invalid host manifest: %w", err)
	}
	if err := decodeStrictJSON(contract.packageRaw, &contract.packageJSON); err != nil {
		return nil, fmt.Errorf("invalid host package.json: %w", err)
	}
	if err := contract.validate(); err != nil {
		return nil, err
	}
	return contract, nil
}

func (contract *hostContract) validate() error {
	if contract.manifest.Name == "" || !strings.HasPrefix(contract.manifest.Name, "apple:bundle-id:") {
		return fmt.Errorf("host manifest name must use apple:bundle-id: URI grammar")
	}
	bundleID := strings.TrimPrefix(contract.manifest.Name, "apple:bundle-id:")
	if contract.packageJSON.Name != bundleID {
		return fmt.Errorf("host package name %q does not match provider %q", contract.packageJSON.Name, contract.manifest.Name)
	}
	if !isSafePackageName(contract.packageJSON.Name) {
		return fmt.Errorf("invalid host package name: %s", contract.packageJSON.Name)
	}
	if contract.manifest.Runtime != "./index.js" || contract.manifest.Types != "./index.d.ts" {
		return fmt.Errorf("host manifest must reference ./index.js and ./index.d.ts")
	}
	if !contract.packageJSON.Private || contract.packageJSON.Type != "module" {
		return fmt.Errorf("host package must be private and use module type")
	}
	if contract.packageJSON.PeerDependencies["@flickfyi/core"] == "" {
		return fmt.Errorf("host package must declare @flickfyi/core as a peer dependency")
	}
	if contract.packageJSON.Types != "./index.d.ts" {
		return fmt.Errorf("host package types must reference ./index.d.ts")
	}
	rootExport, ok := contract.packageJSON.Exports["."]
	if !ok || rootExport.Default != "./index.js" || rootExport.Types != "./index.d.ts" {
		return fmt.Errorf("host package root export must reference ./index.js and ./index.d.ts")
	}
	if len(contract.manifest.Exports) == 0 {
		return fmt.Errorf("host manifest must export at least one scope")
	}
	rootScope := ""
	for contextPath, exported := range contract.manifest.Exports {
		if contextPath == "" || strings.HasPrefix(contextPath, "/") || path.Clean(contextPath) != contextPath {
			return fmt.Errorf("invalid host manifest export path: %s", contextPath)
		}
		if exported.Types != "./index.d.ts" {
			return fmt.Errorf("host manifest export %s must reference ./index.d.ts", contextPath)
		}
		parts := strings.Split(contextPath, "#")
		if len(parts) > 2 || len(parts) == 2 && !isCapabilityFragment(parts[1]) {
			return fmt.Errorf("invalid host capability export path: %s", contextPath)
		}
		if len(parts) == 1 && (rootScope == "" || scopeDepth(contextPath) < scopeDepth(rootScope)) {
			rootScope = contextPath
		}
	}
	if rootScope == "" {
		return fmt.Errorf("host manifest must export at least one activation scope")
	}
	for contextPath := range contract.manifest.Exports {
		scope := strings.SplitN(contextPath, "#", 2)[0]
		if strings.Contains(contextPath, "#") {
			if _, ok := contract.manifest.Exports[scope]; !ok {
				return fmt.Errorf("host capability export %s is missing scope export %s", contextPath, scope)
			}
		}
		if !isSameOrDescendantScope(scope, rootScope) {
			return fmt.Errorf("host manifest export %s is outside root scope %s", contextPath, rootScope)
		}
	}
	return nil
}

func (contract *hostContract) install(fs *diskFS) {
	base := "/node_modules/" + contract.packageJSON.Name + "/"
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.userFiles[base+"index.d.ts"] = string(contract.declarations)
	fs.userFiles[base+"index.js"] = string(contract.runtime)
	fs.userFiles[base+"manifest.json"] = string(contract.manifestRaw)
	fs.userFiles[base+"package.json"] = string(contract.packageRaw)
}

func (contract *hostContract) hasScope(scope string) bool {
	_, ok := contract.manifest.Exports[scope]
	return ok && !strings.Contains(scope, "#")
}

func isFlickCompilationFile(fileName string) bool {
	return fileName == activationOverridePath || strings.HasPrefix(fileName, hostContractPrefix)
}

func isCapabilityFragment(fragment string) bool {
	for _, prefix := range []string{"actions/", "states/", "streams/"} {
		if strings.HasPrefix(fragment, prefix) {
			name := strings.TrimPrefix(fragment, prefix)
			return name != "" && !strings.HasPrefix(name, "/") && path.Clean(name) == name
		}
	}
	return false
}

func isHostContractFile(fileName string) bool {
	for _, name := range hostContractFileNames {
		if fileName == hostContractPrefix+name {
			return true
		}
	}
	return false
}

func isSameOrDescendantScope(scope string, ancestor string) bool {
	return scope == ancestor || strings.HasPrefix(scope, ancestor+"/")
}

func scopeDepth(scope string) int {
	return strings.Count(scope, "/")
}

func isSafePackageName(name string) bool {
	return !strings.HasPrefix(name, ".") && hostPackageNamePattern.MatchString(name)
}

func decodeStrictJSON(raw []byte, value any) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}
