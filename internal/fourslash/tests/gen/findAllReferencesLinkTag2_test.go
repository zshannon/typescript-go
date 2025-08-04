package fourslash_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/fourslash"
	"github.com/microsoft/typescript-go/internal/testutil"
)

func TestFindAllReferencesLinkTag2(t *testing.T) {
	t.Parallel()

	defer testutil.RecoverAndFail(t, "Panic on fourslash test")
	const content = `namespace NPR/*5*/ {
    export class Consider/*4*/ {
        This/*3*/ = class {
            show/*2*/() { }
        }
        m/*1*/() { }
    }
    /**
     * @see {Consider.prototype.m}
     * {@link Consider#m}
     * @see {Consider#This#show}
     * {@link Consider.This.show}
     * @see {NPR.Consider#This#show}
     * {@link NPR.Consider.This#show}
     * @see {NPR.Consider#This.show} # doesn't parse trailing .
     * @see {NPR.Consider.This.show}
     */
    export function ref() { }
}
/**
 * {@link NPR.Consider#This#show hello hello}
 * {@link NPR.Consider.This#show}
 * @see {NPR.Consider#This.show} # doesn't parse trailing .
 * @see {NPR.Consider.This.show}
 */
export function outerref() { }`
	f := fourslash.NewFourslash(t, nil /*capabilities*/, content)
	f.VerifyBaselineFindAllReferences(t, "1", "2", "3", "4", "5")
}
