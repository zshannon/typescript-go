package fourslash_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/fourslash"
	"github.com/microsoft/typescript-go/internal/testutil"
)

func TestReferencesForExpressionKeywords(t *testing.T) {
	t.Parallel()

	defer testutil.RecoverAndFail(t, "Panic on fourslash test")
	const content = `class C {
    static x = 1;
}
/*new*/new C();
/*void*/void C;
/*typeof*/typeof C;
/*delete*/delete C.x;
/*async*/async function* f() {
    /*yield*/yield C;
    /*await*/await C;
}
"x" /*in*/in C;
undefined /*instanceof*/instanceof C;
undefined /*as*/as C;`
	f := fourslash.NewFourslash(t, nil /*capabilities*/, content)
	f.VerifyBaselineFindAllReferences(t, "new", "void", "typeof", "yield", "await", "in", "instanceof", "as", "delete")
}
