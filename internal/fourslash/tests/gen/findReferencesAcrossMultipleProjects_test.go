package fourslash_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/fourslash"
	"github.com/microsoft/typescript-go/internal/testutil"
)

func TestFindReferencesAcrossMultipleProjects(t *testing.T) {
	t.Parallel()

	defer testutil.RecoverAndFail(t, "Panic on fourslash test")
	const content = `//@Filename: a.ts
/*1*/var /*2*/x: number;
//@Filename: b.ts
/// <reference path="a.ts" />
/*3*/x++;
//@Filename: c.ts
/// <reference path="a.ts" />
/*4*/x++;`
	f := fourslash.NewFourslash(t, nil /*capabilities*/, content)
	f.VerifyBaselineFindAllReferences(t, "1", "2", "3", "4")
}
