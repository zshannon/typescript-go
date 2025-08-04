package fourslash_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/fourslash"
	"github.com/microsoft/typescript-go/internal/testutil"
)

func TestFindAllRefsOnDefinition2(t *testing.T) {
	t.Parallel()

	defer testutil.RecoverAndFail(t, "Panic on fourslash test")
	const content = `//@Filename: findAllRefsOnDefinition2-import.ts
export module Test{

    /*1*/export interface /*2*/start { }

    export interface stop { }
}
//@Filename: findAllRefsOnDefinition2.ts
import Second = require("./findAllRefsOnDefinition2-import");

var start: Second.Test./*3*/start;
var stop: Second.Test.stop;`
	f := fourslash.NewFourslash(t, nil /*capabilities*/, content)
	f.VerifyBaselineFindAllReferences(t, "1", "2", "3")
}
