package fourslash_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/fourslash"
	"github.com/microsoft/typescript-go/internal/testutil"
)

func TestFindAllRefsRedeclaredPropertyInDerivedInterface(t *testing.T) {
	t.Parallel()

	defer testutil.RecoverAndFail(t, "Panic on fourslash test")
	const content = `// @noLib: true
interface A {
    readonly /*0*/x: number | string;
}
interface B extends A {
    readonly /*1*/x: number;
}
const a: A = { /*2*/x: 0 };
const b: B = { /*3*/x: 0 };`
	f := fourslash.NewFourslash(t, nil /*capabilities*/, content)
	f.VerifyBaselineFindAllReferences(t, "0", "1", "2", "3")
}
