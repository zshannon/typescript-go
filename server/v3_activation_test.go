package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/compiler"
)

const activationProvider = "apple:bundle-id:fyi.flick.test-host"
const activationPackage = "fyi.flick.test-host"

func TestDeriveActivationFollowsReachableAliasesAndIgnoresUnusedFiles(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `
import { messages as useMessages } from "./messages"
export default function App() {
	useMessages()
	return null
}
`,
		"/messages.ts": `
import { useContextState as useHostState } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
export function messages() {
	return useHostState(Application.conversation.states.messages)
}
`,
		"/unused.ts": `
import { useContextAction } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
export function unused() {
	return useContextAction(Application.settings.actions.reset)
}
`,
	})
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	result, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) > 0 {
		t.Fatalf("activation failed: %v", errors)
	}
	if got, want := result.Explanation.ActivationScope, activationProvider+"/application/conversation"; got != want {
		t.Fatalf("activation scope = %q, want %q", got, want)
	}
	if len(result.Explanation.References) != 1 {
		t.Fatalf("references = %d, want 1: %#v", len(result.Explanation.References), result.Explanation.References)
	}
	reference := result.Explanation.References[0]
	if reference.Context != "application/conversation#states/messages" || !reference.Required {
		t.Fatalf("unexpected reference: %#v", reference)
	}
	if reference.Span.File != "/messages.ts" || reference.Span.Start.Line != 5 {
		t.Fatalf("unexpected source span: %#v", reference.Span)
	}
	requirement := result.Manifest.Contexts[activationProvider][reference.Context]
	if !requirement.Required {
		t.Fatalf("manifest requirement = %#v, want required", requirement)
	}
	if result.Manifest.Dependencies[activationProvider] != "*" {
		t.Fatalf("dependency version = %q, want *", result.Manifest.Dependencies[activationProvider])
	}
}

func TestDeriveActivationSelectsRequiredRootScope(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `
import { useContextState } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
export default function App() {
	useContextState(Application.states.currentMember)
	return null
}
`,
	})
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	result, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) > 0 {
		t.Fatalf("activation failed: %v", errors)
	}
	if got, want := result.Explanation.ActivationScope, activationProvider+"/application"; got != want {
		t.Fatalf("activation scope = %q, want %q", got, want)
	}
}

func TestDeriveActivationSelectsDeepestAncestorDescendantScope(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `
import { useContextState } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
export default function App() {
	useContextState(Application.states.currentMember)
	useContextState(Application.conversation.states.messages)
	return null
}
`,
	})
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	result, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) > 0 {
		t.Fatalf("activation failed: %v", errors)
	}
	if got, want := result.Explanation.ActivationScope, activationProvider+"/application/conversation"; got != want {
		t.Fatalf("activation scope = %q, want %q", got, want)
	}
	if len(result.Explanation.References) != 2 {
		t.Fatalf("references = %d, want 2", len(result.Explanation.References))
	}
}

func TestDeriveActivationFollowsStaticJSXComponents(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/card.tsx": `
import { useContextObservation } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
export const Card = () => {
	useContextObservation(Application.conversation.states.messages)
	return null
}
`,
		"/index.tsx": `
import { Card as ConversationCard } from "./card"
export default function App() {
	return <ConversationCard />
}
`,
	})
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	result, errors := deriveActivation(context.Background(), program, "./index.tsx", hostContext)
	if len(errors) > 0 {
		t.Fatalf("activation failed: %v", errors)
	}
	if len(result.Explanation.References) != 1 || result.Explanation.References[0].Hook != "useContextObservation" {
		t.Fatalf("unexpected JSX references: %#v", result.Explanation.References)
	}
}

func TestDeriveActivationFollowsWrappedDefaultComponent(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `
import { useContextState } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
function memo<Component>(component: Component): Component {
	return component
}
function App() {
	useContextState(Application.conversation.states.messages)
	return null
}
export default memo(App)
`,
	})
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	result, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) > 0 {
		t.Fatalf("activation failed: %v", errors)
	}
	if len(result.Explanation.References) != 1 {
		t.Fatalf("wrapped component references = %d, want 1", len(result.Explanation.References))
	}
}

func TestDeriveActivationFollowsCallableArguments(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `
import { useContextState } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
function invoke(callback: () => void) {
	callback()
}
function Conversation() {
	useContextState(Application.conversation.states.messages)
}
export default function App() {
	invoke(Conversation)
	return null
}
`,
	})
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	result, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) > 0 {
		t.Fatalf("activation failed: %v", errors)
	}
	if len(result.Explanation.References) != 1 {
		t.Fatalf("callable argument references = %d, want 1", len(result.Explanation.References))
	}
	if got, want := result.Explanation.References[0].Context, "application/conversation#states/messages"; got != want {
		t.Fatalf("callable argument context = %q, want %q", got, want)
	}
}

func TestDeriveActivationDoesNotDuplicateNestedHookCalls(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `
import { useContextState } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
function consume(value: unknown) {
	return value
}
export default function App() {
	consume(useContextState(Application.conversation.states.messages))
	return null
}
`,
	})
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	result, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) > 0 {
		t.Fatalf("activation failed: %v", errors)
	}
	if len(result.Explanation.References) != 1 {
		t.Fatalf("nested hook references = %d, want 1", len(result.Explanation.References))
	}
}

func TestDeriveActivationReportsSpanEndingAtEOF(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `import { useContextState } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
export default () => useContextState(Application.conversation.states.messages)`,
	})
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	result, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) > 0 {
		t.Fatalf("activation failed: %v", errors)
	}
	if len(result.Explanation.References) != 1 {
		t.Fatalf("references = %d, want 1", len(result.Explanation.References))
	}
	span := result.Explanation.References[0].Span
	if span.End.Line != 3 || span.End.Column <= span.Start.Column {
		t.Fatalf("EOF span end = %#v, start = %#v", span.End, span.Start)
	}
}

func TestDeriveActivationFollowsLocalDefaultExportAlias(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `
import { useContextState } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
function App() {
	useContextState(Application.conversation.states.messages)
	return null
}
export { App as default }
`,
	})
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	result, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) > 0 {
		t.Fatalf("activation failed: %v", errors)
	}
	if len(result.Explanation.References) != 1 {
		t.Fatalf("local default alias references = %d, want 1", len(result.Explanation.References))
	}
}

func TestDeriveActivationFollowsDefaultReexport(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/App.ts": `
import { useContextState } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
export default function App() {
	useContextState(Application.conversation.states.messages)
	return null
}
`,
		"/index.ts": `export { default } from "./App"`,
	})
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	result, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) > 0 {
		t.Fatalf("activation failed: %v", errors)
	}
	if len(result.Explanation.References) != 1 {
		t.Fatalf("default reexport references = %d, want 1", len(result.Explanation.References))
	}
}

func TestDeriveActivationRejectsDynamicAddress(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `
import { type ContextState, useContextState } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
const dynamicAddress: ContextState<string, string> = Application.conversation.states.messages
export default function App() {
	useContextState(dynamicAddress)
	return null
}
`,
	})
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	_, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) != 1 || errors[0].Message != "useContextState address must resolve to one literal host context URI" {
		t.Fatalf("unexpected dynamic-address errors: %#v", errors)
	}
}

func TestDeriveActivationUsesValidatedOverride(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `export default function App() { return null }`,
	})
	files[activationOverridePath] = []byte(`{"scope":"application/settings"}`)
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	result, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) > 0 {
		t.Fatalf("activation failed: %v", errors)
	}
	if got, want := result.Explanation.ActivationScope, activationProvider+"/application/settings"; got != want {
		t.Fatalf("activation scope = %q, want %q", got, want)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"references":[]`) {
		t.Fatalf("empty references must encode as an array: %s", encoded)
	}
}

func TestDeriveActivationOptionalReferenceKeepsRootScope(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `
import { useContextState } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
export default function App() {
	useContextState(Application.conversation.states.messages, { optional: true })
	return null
}
`,
	})
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	result, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) > 0 {
		t.Fatalf("activation failed: %v", errors)
	}
	if got, want := result.Explanation.ActivationScope, activationProvider+"/application"; got != want {
		t.Fatalf("activation scope = %q, want %q", got, want)
	}
	reference := result.Explanation.References[0]
	if reference.Required {
		t.Fatalf("optional reference marked required: %#v", reference)
	}
	if result.Manifest.Contexts[activationProvider][reference.Context].Required {
		t.Fatalf("optional manifest context marked required")
	}
}

func TestDeriveActivationIncludesRequiredStreamObservation(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `
import { useContextObservation } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
export default function App() {
	useContextObservation(Application.conversation.streams.events)
	return null
}
`,
	})
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	result, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) > 0 {
		t.Fatalf("activation failed: %v", errors)
	}
	if got, want := result.Explanation.ActivationScope, activationProvider+"/application/conversation"; got != want {
		t.Fatalf("activation scope = %q, want %q", got, want)
	}
	reference := result.Explanation.References[0]
	if reference.Context != "application/conversation#streams/events" || !reference.Required {
		t.Fatalf("unexpected stream reference: %#v", reference)
	}
}

func TestDeriveActivationRejectsNonliteralOptionalFlag(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `
import { useContextState } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
declare const optional: boolean
export default function App() {
	useContextState(Application.conversation.states.messages, { optional })
	return null
}
`,
	})
	_, _, response := typecheckActivationRequest(t, files)
	if len(response.Errors) == 0 {
		t.Fatal("expected a nonliteral optional flag to fail typechecking")
	}
}

func TestDeriveActivationRejectsSiblingScopes(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `
import { useContextAction, useContextState } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
export default function App() {
	useContextState(Application.conversation.states.messages)
	useContextAction(Application.settings.actions.reset)
	return null
}
`,
	})
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	_, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) != 1 || errors[0].Message != "required host contexts have ambiguous sibling activation scopes: application/conversation and application/settings" {
		t.Fatalf("unexpected activation errors: %#v", errors)
	}
}

func TestDeriveActivationUsesOverrideForSiblingScopes(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `
import { useContextAction, useContextState } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
export default function App() {
	useContextState(Application.conversation.states.messages)
	useContextAction(Application.settings.actions.reset)
	return null
}
`,
	})
	files[activationOverridePath] = []byte(`{"scope":"application"}`)
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	result, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) > 0 {
		t.Fatalf("activation failed: %v", errors)
	}
	if got, want := result.Explanation.ActivationScope, activationProvider+"/application"; got != want {
		t.Fatalf("activation scope = %q, want %q", got, want)
	}
	if len(result.Explanation.References) != 2 {
		t.Fatalf("references = %d, want 2", len(result.Explanation.References))
	}
}

func TestDeriveActivationRejectsHookCapabilityMismatch(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `
import { useContextAction } from "@flickfyi/core"
import { Application } from "fyi.flick.test-host"
export default function App() {
	useContextAction(Application.conversation.states.messages)
	return null
}
`,
	})
	program, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("typecheck failed: %v", response.Errors)
	}

	_, errors := deriveActivation(context.Background(), program, "./index.ts", hostContext)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "useContextAction cannot consume host capability") {
		t.Fatalf("unexpected hook mismatch errors: %#v", errors)
	}
}

func TestHostContractRejectsUnknownCapabilityGrammar(t *testing.T) {
	files := activationRequestFiles(map[string]string{"/index.ts": "export default null"})
	host := activationHostContract()
	var manifest map[string]any
	if err := json.Unmarshal([]byte(host["manifest.json"]), &manifest); err != nil {
		t.Fatal(err)
	}
	exports := manifest["exports"].(map[string]any)
	exports["application#values/unsupported"] = map[string]string{"types": "./index.d.ts"}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	files[hostContractPrefix+"manifest.json"] = encoded

	_, err = parseHostCompilationContext(files)
	if err == nil || !strings.Contains(err.Error(), "invalid host capability export path") {
		t.Fatalf("unexpected host validation error: %v", err)
	}
}

func TestHostContractRejectsInvalidTrustBoundary(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]string)
		want   string
	}{
		{
			name: "partial contract",
			mutate: func(host map[string]string) {
				delete(host, "index.js")
			},
			want: "host contract requires",
		},
		{
			name: "unexpected file",
			mutate: func(host map[string]string) {
				host["extra.js"] = "export {}"
			},
			want: "unexpected host contract file",
		},
		{
			name: "empty runtime",
			mutate: func(host map[string]string) {
				host["index.js"] = ""
			},
			want: "runtime and declarations must not be empty",
		},
		{
			name: "unknown manifest field",
			mutate: func(host map[string]string) {
				host["manifest.json"] = strings.TrimSuffix(host["manifest.json"], "}") + `,"unknown":true}`
			},
			want: "unknown field",
		},
		{
			name: "unknown package field",
			mutate: func(host map[string]string) {
				host["package.json"] = strings.TrimSuffix(strings.TrimSpace(host["package.json"]), "}") + `,"unknown":true}`
			},
			want: "unknown field",
		},
		{
			name: "provider package mismatch",
			mutate: func(host map[string]string) {
				host["package.json"] = strings.Replace(host["package.json"], activationPackage, "fyi.flick.other-host", 1)
			},
			want: "does not match provider",
		},
		{
			name: "unsafe package name",
			mutate: func(host map[string]string) {
				host["manifest.json"] = strings.Replace(host["manifest.json"], activationProvider, "apple:bundle-id:.hidden", 1)
				host["package.json"] = strings.Replace(host["package.json"], activationPackage, ".hidden", 1)
			},
			want: "invalid host package name",
		},
		{
			name: "non-generated package grammar",
			mutate: func(host map[string]string) {
				host["manifest.json"] = strings.Replace(host["manifest.json"], activationProvider, "apple:bundle-id:fyi.flick:test-host", 1)
				host["package.json"] = strings.Replace(host["package.json"], activationPackage, "fyi.flick:test-host", 1)
			},
			want: "invalid host package name",
		},
		{
			name: "wrong manifest runtime",
			mutate: func(host map[string]string) {
				host["manifest.json"] = strings.Replace(host["manifest.json"], `"runtime":"./index.js"`, `"runtime":"./other.js"`, 1)
			},
			want: "host manifest must reference",
		},
		{
			name: "public package",
			mutate: func(host map[string]string) {
				host["package.json"] = strings.Replace(host["package.json"], `"private": true`, `"private": false`, 1)
			},
			want: "host package must be private",
		},
		{
			name: "missing core peer dependency",
			mutate: func(host map[string]string) {
				host["package.json"] = strings.Replace(host["package.json"], `"peerDependencies": {"@flickfyi/core": "*"}`, `"peerDependencies": {}`, 1)
			},
			want: "must declare @flickfyi/core",
		},
		{
			name: "wrong root export",
			mutate: func(host map[string]string) {
				host["package.json"] = strings.Replace(host["package.json"], `"default": "./index.js"`, `"default": "./other.js"`, 1)
			},
			want: "host package root export must reference",
		},
		{
			name: "capability missing scope",
			mutate: func(host map[string]string) {
				rewriteActivationHostJSON(t, host, "manifest.json", func(value map[string]any) {
					delete(value["exports"].(map[string]any), "application/settings")
				})
			},
			want: "is missing scope export",
		},
		{
			name: "sibling root",
			mutate: func(host map[string]string) {
				rewriteActivationHostJSON(t, host, "manifest.json", func(value map[string]any) {
					value["exports"].(map[string]any)["settings"] = map[string]string{"types": "./index.d.ts"}
				})
			},
			want: "outside root scope",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := activationRequestFiles(map[string]string{"/index.ts": "export default null"})
			for _, fileName := range hostContractFileNames {
				delete(files, hostContractPrefix+fileName)
			}
			host := activationHostContract()
			test.mutate(host)
			for fileName, content := range host {
				files[hostContractPrefix+fileName] = []byte(content)
			}

			_, err := parseHostCompilationContext(files)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("host validation error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestActivationOverrideRejectsInvalidTrustBoundary(t *testing.T) {
	tests := []struct {
		name     string
		override string
		withHost bool
		want     string
	}{
		{name: "without host", override: `{"scope":"application"}`, want: "requires a host contract"},
		{name: "unknown field", override: `{"scope":"application","unknown":true}`, withHost: true, want: "unknown field"},
		{name: "missing scope", override: `{}`, withHost: true, want: "missing required field: scope"},
		{name: "capability instead of scope", override: `{"scope":"application#states/currentMember"}`, withHost: true, want: "scope is not exported"},
		{name: "unexported scope", override: `{"scope":"application/profile"}`, withHost: true, want: "scope is not exported"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := map[string][]byte{activationOverridePath: []byte(test.override)}
			if test.withHost {
				for fileName, content := range activationHostContract() {
					files[hostContractPrefix+fileName] = []byte(content)
				}
			}

			_, err := parseHostCompilationContext(files)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("activation override error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestHostContractIsRequestLocal(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `import { Application } from "fyi.flick.test-host"
export const value = Application.conversation.states.messages`,
	})
	_, hostContext, response := typecheckActivationRequest(t, files)
	if len(response.Errors) > 0 {
		t.Fatalf("host typecheck failed: %v", response.Errors)
	}
	if hostContext.contract == nil {
		t.Fatal("expected parsed host contract")
	}

	var program *compiler.Program
	withoutHost := typecheckV3WithHost(
		context.Background(),
		withoutHostCompilationFiles(files),
		files["/tsconfig.json"],
		[]byte("activation-test-lock"),
		nil,
		&program,
	)
	if len(withoutHost.Errors) == 0 {
		t.Fatal("host package leaked into a later request")
	}
}

func TestCompileV3LoadsHostRuntime(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `import { Application } from "fyi.flick.test-host"
export const value = Application.conversation.states.messages`,
	})
	setupActivationDependencyCache(t, []byte("activation-test-lock"))
	hostContext, err := parseHostCompilationContext(files)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := parsePackageJSON(files["/package.json"])
	if err != nil {
		t.Fatal(err)
	}

	response := compileV3WithHost(
		context.Background(),
		files,
		pkg,
		files["/tsconfig.json"],
		[]byte("activation-test-lock"),
		hostContext.contract,
	)
	if len(response.Errors) > 0 {
		t.Fatalf("host compile failed: %v", response.Errors)
	}
	if !strings.Contains(response.Code, "apple:bundle-id:fyi.flick.test-host/application/conversation#states/messages") {
		t.Fatalf("compiled code did not include the host runtime: %s", response.Code)
	}
}

func TestCompileV3BundlesHostRuntimeWhenListedExternal(t *testing.T) {
	files := activationRequestFiles(map[string]string{
		"/index.ts": `import { Application } from "fyi.flick.test-host"
export const value = Application.conversation.states.messages`,
	})
	setupActivationDependencyCache(t, []byte("activation-test-lock"))
	hostContext, err := parseHostCompilationContext(files)
	if err != nil {
		t.Fatal(err)
	}

	for _, external := range []string{activationPackage, "*"} {
		t.Run(external, func(t *testing.T) {
			pkg, err := parsePackageJSON(files["/package.json"])
			if err != nil {
				t.Fatal(err)
			}
			pkg.Esbuild.External = []string{external}

			response := compileV3WithHost(
				context.Background(),
				files,
				pkg,
				files["/tsconfig.json"],
				[]byte("activation-test-lock"),
				hostContext.contract,
			)
			if len(response.Errors) > 0 {
				t.Fatalf("host compile failed: %v", response.Errors)
			}
			if !strings.Contains(response.Code, "apple:bundle-id:fyi.flick.test-host/application/conversation#states/messages") {
				t.Fatalf("compiled code did not include the host runtime: %s", response.Code)
			}
		})
	}
}

func typecheckActivationRequest(t *testing.T, files map[string][]byte) (*compiler.Program, *hostCompilationContext, TypecheckV2Response) {
	t.Helper()
	setupActivationDependencyCache(t, []byte("activation-test-lock"))
	hostContext, err := parseHostCompilationContext(files)
	if err != nil {
		t.Fatal(err)
	}
	var program *compiler.Program
	response := typecheckV3WithHost(
		context.Background(),
		files,
		files["/tsconfig.json"],
		[]byte("activation-test-lock"),
		hostContext.contract,
		&program,
	)
	return program, hostContext, response
}

func setupActivationDependencyCache(t *testing.T, lockContent []byte) {
	t.Helper()
	oldCachePath := diskCachePath
	diskCachePath = t.TempDir()
	t.Cleanup(func() {
		diskCachePath = oldCachePath
	})

	coreDirectory := filepath.Join(
		diskCachePath,
		"deps",
		hashBunLock(lockContent),
		"node_modules",
		"@flickfyi",
		"core",
	)
	if err := os.MkdirAll(coreDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	writeActivationFixture(t, filepath.Join(coreDirectory, "package.json"), `{
		"name": "@flickfyi/core",
		"types": "./index.d.ts",
		"main": "./index.js",
		"exports": {".": {"types": "./index.d.ts", "default": "./index.js"}, "./jsx-runtime": {"types": "./jsx-runtime.d.ts", "default": "./jsx-runtime.js"}}
	}`)
	writeActivationFixture(t, filepath.Join(coreDirectory, "index.d.ts"), `
export type ContextAddress<URI extends string> = Readonly<{ uri: URI }>
export type ContextAction<URI extends string, Input, Output> = ContextAddress<URI>
export type ContextObservation<URI extends string, Value, Failure = never, Requirements = never> = ContextAddress<URI>
export type ContextState<URI extends string, Value, Failure = never, Requirements = never> = ContextAddress<URI>
export function useContextAction<Address extends ContextAction<string, unknown, unknown>>(address: Address, options: { optional: true }): unknown | undefined
export function useContextAction<Address extends ContextAction<string, unknown, unknown>>(address: Address, options?: { optional?: false }): unknown
export function useContextObservation<Address extends ContextObservation<string, unknown, unknown, unknown>>(address: Address, options: { optional: true }): unknown | undefined
export function useContextObservation<Address extends ContextObservation<string, unknown, unknown, unknown>>(address: Address, options?: { optional?: false }): unknown
export function useContextState<Address extends ContextState<string, unknown, unknown, unknown>>(address: Address, options: { optional: true }): unknown | undefined
export function useContextState<Address extends ContextState<string, unknown, unknown, unknown>>(address: Address, options?: { optional?: false }): unknown
`)
	writeActivationFixture(t, filepath.Join(coreDirectory, "index.js"), `
export const useContextAction = () => undefined
export const useContextObservation = () => undefined
export const useContextState = () => undefined
`)
	writeActivationFixture(t, filepath.Join(coreDirectory, "jsx-runtime.d.ts"), `
export namespace JSX {
	interface Element {}
	interface IntrinsicElements { [name: string]: unknown }
}
export function jsx(type: unknown, props: unknown, key?: unknown): JSX.Element
export function jsxs(type: unknown, props: unknown, key?: unknown): JSX.Element
export function Fragment(props: unknown): JSX.Element
`)
	writeActivationFixture(t, filepath.Join(coreDirectory, "jsx-runtime.js"), `
export const jsx = (type, props) => ({ type, props })
export const jsxs = jsx
export const Fragment = ({ children }) => children
`)
}

func activationRequestFiles(sources map[string]string) map[string][]byte {
	files := map[string][]byte{
		"/bun.lock":      []byte("activation-test-lock"),
		"/package.json":  []byte(`{"main":"./index.ts","dependencies":{}}`),
		"/tsconfig.json": []byte(`{"compilerOptions":{"jsx":"react-jsx","jsxImportSource":"@flickfyi/core"}}`),
	}
	for path, source := range sources {
		files[path] = []byte(source)
	}
	host := activationHostContract()
	for name, content := range host {
		files[hostContractPrefix+name] = []byte(content)
	}
	return files
}

func activationHostContract() map[string]string {
	manifest := map[string]any{
		"exports": map[string]any{
			"application":                              map[string]string{"types": "./index.d.ts"},
			"application#states/currentMember":         map[string]string{"types": "./index.d.ts"},
			"application/conversation":                 map[string]string{"types": "./index.d.ts"},
			"application/conversation#states/messages": map[string]string{"types": "./index.d.ts"},
			"application/conversation#streams/events":  map[string]string{"types": "./index.d.ts"},
			"application/settings":                     map[string]string{"types": "./index.d.ts"},
			"application/settings#actions/reset":       map[string]string{"types": "./index.d.ts"},
		},
		"name":    activationProvider,
		"runtime": "./index.js",
		"types":   "./index.d.ts",
	}
	manifestJSON, _ := json.Marshal(manifest)
	return map[string]string{
		"index.d.ts": `
import type { ContextAction, ContextObservation, ContextState } from "@flickfyi/core"
export const Application: {
	readonly states: {
		readonly currentMember: ContextState<"apple:bundle-id:fyi.flick.test-host/application#states/currentMember", string>
	}
	readonly conversation: {
		readonly states: {
			readonly messages: ContextState<"apple:bundle-id:fyi.flick.test-host/application/conversation#states/messages", string>
		}
		readonly streams: {
			readonly events: ContextObservation<"apple:bundle-id:fyi.flick.test-host/application/conversation#streams/events", string>
		}
	}
	readonly settings: {
		readonly actions: {
			readonly reset: ContextAction<"apple:bundle-id:fyi.flick.test-host/application/settings#actions/reset", void, void>
		}
	}
}
`,
		"index.js": `
export const Application = {
	states: { currentMember: { uri: "apple:bundle-id:fyi.flick.test-host/application#states/currentMember" } },
	conversation: {
		states: { messages: { uri: "apple:bundle-id:fyi.flick.test-host/application/conversation#states/messages" } },
		streams: { events: { uri: "apple:bundle-id:fyi.flick.test-host/application/conversation#streams/events" } }
	},
	settings: { actions: { reset: { uri: "apple:bundle-id:fyi.flick.test-host/application/settings#actions/reset" } } }
}
`,
		"manifest.json": string(manifestJSON),
		"package.json": `{
			"exports": {".": {"default": "./index.js", "types": "./index.d.ts"}},
			"name": "fyi.flick.test-host",
			"peerDependencies": {"@flickfyi/core": "*"},
			"private": true,
			"type": "module",
			"types": "./index.d.ts"
		}`,
	}
}

func withoutHostCompilationFiles(files map[string][]byte) map[string][]byte {
	result := make(map[string][]byte)
	for path, content := range files {
		if !isFlickCompilationFile(path) {
			result[path] = content
		}
	}
	return result
}

func rewriteActivationHostJSON(t *testing.T, host map[string]string, fileName string, mutate func(map[string]any)) {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(host[fileName]), &value); err != nil {
		t.Fatal(err)
	}
	mutate(value)
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	host[fileName] = string(encoded)
}

func writeActivationFixture(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
