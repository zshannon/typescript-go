package fourslash_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/fourslash"
	"github.com/microsoft/typescript-go/internal/testutil"
)

func TestFindAllRefsRootSymbols(t *testing.T) {
	t.Parallel()

	defer testutil.RecoverAndFail(t, "Panic on fourslash test")
	const content = `interface I { /*0*/x: {}; }
interface J { /*1*/x: {}; }
declare const o: (I | J) & { /*2*/x: string };
o./*3*/x;`
	f := fourslash.NewFourslash(t, nil /*capabilities*/, content)
	f.VerifyBaselineFindAllReferences(t, "0", "1", "2", "3")
}
