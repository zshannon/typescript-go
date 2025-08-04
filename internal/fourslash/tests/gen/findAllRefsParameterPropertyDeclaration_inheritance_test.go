package fourslash_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/fourslash"
	"github.com/microsoft/typescript-go/internal/testutil"
)

func TestFindAllRefsParameterPropertyDeclaration_inheritance(t *testing.T) {
	t.Parallel()

	defer testutil.RecoverAndFail(t, "Panic on fourslash test")
	const content = `class C {
	constructor(public /*0*/x: string) {
		/*1*/x;
	}
}
class D extends C {
	constructor(public /*2*/x: string) {
		super(/*3*/x);
	}
}`
	f := fourslash.NewFourslash(t, nil /*capabilities*/, content)
	f.VerifyBaselineFindAllReferences(t, "0", "1", "2", "3")
}
