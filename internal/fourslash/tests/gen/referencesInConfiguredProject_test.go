package fourslash_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/fourslash"
	"github.com/microsoft/typescript-go/internal/testutil"
)

func TestReferencesInConfiguredProject(t *testing.T) {
	t.Parallel()

	defer testutil.RecoverAndFail(t, "Panic on fourslash test")
	const content = `// @Filename: /home/src/workspaces/project/referencesForGlobals_1.ts
class /*0*/globalClass {
    public f() { }
}
// @Filename: /home/src/workspaces/project/referencesForGlobals_2.ts
var c = /*1*/globalClass();
// @Filename: /home/src/workspaces/project/tsconfig.json
{ "files": ["referencesForGlobals_1.ts", "referencesForGlobals_2.ts"] }`
	f := fourslash.NewFourslash(t, nil /*capabilities*/, content)
	f.VerifyBaselineFindAllReferences(t, "1")
}
