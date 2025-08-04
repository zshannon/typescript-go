package fourslash_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/fourslash"
	"github.com/microsoft/typescript-go/internal/testutil"
)

func TestUpdateToClassStatics(t *testing.T) {
	t.Parallel()

	defer testutil.RecoverAndFail(t, "Panic on fourslash test")
	const content = `module TypeScript {
    export class PullSymbol {}
    export class Diagnostic {}
    export class SymbolAndDiagnostics<TSymbol extends PullSymbol> {
        constructor(public symbol: TSymbol,
            public diagnostics: Diagnostic) {
        }
        /**/
        public static create<TSymbol extends PullSymbol>(symbol: TSymbol, diagnostics: Diagnostic): SymbolAndDiagnostics<TSymbol> {
            return new SymbolAndDiagnostics<TSymbol>(symbol, diagnostics);
        }
    }
}
module TypeScript {
    var x : TypeScript.SymbolAndDiagnostics;
}`
	f := fourslash.NewFourslash(t, nil /*capabilities*/, content)
	f.GoToMarker(t, "")
	f.Insert(t, "someNewProperty = 0;")
}
