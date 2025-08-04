package fourslash_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/fourslash"
	"github.com/microsoft/typescript-go/internal/testutil"
)

func TestFindAllRefsJsDocImportTag3(t *testing.T) {
	t.Parallel()

	defer testutil.RecoverAndFail(t, "Panic on fourslash test")
	const content = `// @checkJs: true
// @Filename: /component.js
export class Component {
  constructor() {
    this.id_ = Math.random();
  }
  id() {
    return this.id_;
  }
}
// @Filename: /spatial-navigation.js
/** @import { Component } from './component.js' */

export class SpatialNavigation {
  /**
   * @param {Component} component
   */
  add(component) {}
}
// @Filename: /player.js
import { Component } from './component.js';

/**
 * @extends Component/*1*/
 */
export class Player extends Component {}`
	f := fourslash.NewFourslash(t, nil /*capabilities*/, content)
	f.VerifyBaselineFindAllReferences(t, "1")
}
