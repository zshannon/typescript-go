package fourslash_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/fourslash"
	"github.com/microsoft/typescript-go/internal/testutil"
)

func TestReferenceToClass(t *testing.T) {
	t.Parallel()

	defer testutil.RecoverAndFail(t, "Panic on fourslash test")
	const content = `// @Filename: referenceToClass_1.ts
class /*1*/foo {
    public n: /*2*/foo;
    public foo: number;
}

class bar {
    public n: /*3*/foo;
    public k = new /*4*/foo();
}

module mod {
    var k: /*5*/foo = null;
}
// @Filename: referenceToClass_2.ts
var k: /*6*/foo;`
	f := fourslash.NewFourslash(t, nil /*capabilities*/, content)
	f.VerifyBaselineFindAllReferences(t, "1", "2", "3", "4", "5", "6")
}
