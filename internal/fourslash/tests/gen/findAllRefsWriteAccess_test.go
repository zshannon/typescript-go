package fourslash_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/fourslash"
	"github.com/microsoft/typescript-go/internal/testutil"
)

func TestFindAllRefsWriteAccess(t *testing.T) {
	t.Parallel()

	defer testutil.RecoverAndFail(t, "Panic on fourslash test")
	const content = `interface Obj {
    [` + "`" + `/*1*/num` + "`" + `]: number;
}

let o: Obj = {
    [` + "`" + `num` + "`" + `]: 0
};

o = {
    ['num']: 1
};

o['num'] = 2;
o[` + "`" + `num` + "`" + `] = 3;

o['num'];
o[` + "`" + `num` + "`" + `];`
	f := fourslash.NewFourslash(t, nil /*capabilities*/, content)
	f.VerifyBaselineFindAllReferences(t, "1")
}
