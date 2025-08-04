package fourslash_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/fourslash"
	"github.com/microsoft/typescript-go/internal/testutil"
)

func TestReferencesForModifiers(t *testing.T) {
	t.Parallel()

	defer testutil.RecoverAndFail(t, "Panic on fourslash test")
	const content = `[|/*declareModifier*/declare /*abstractModifier*/abstract class C1 {
    [|/*staticModifier*/static a;|]
    [|/*readonlyModifier*/readonly b;|]
    [|/*publicModifier*/public c;|]
    [|/*protectedModifier*/protected d;|]
    [|/*privateModifier*/private e;|]
}|]
[|/*constModifier*/const enum E {
}|]
[|/*asyncModifier*/async function fn() {}|]
[|/*exportModifier*/export /*defaultModifier*/default class C2 {}|]`
	f := fourslash.NewFourslash(t, nil /*capabilities*/, content)
	f.VerifyBaselineFindAllReferences(t, "declareModifier", "abstractModifier", "staticModifier", "readonlyModifier", "publicModifier", "protectedModifier", "privateModifier", "constModifier", "asyncModifier", "exportModifier", "defaultModifier")
}
