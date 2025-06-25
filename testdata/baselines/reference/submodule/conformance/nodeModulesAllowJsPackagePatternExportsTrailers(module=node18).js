//// [tests/cases/conformance/node/allowJs/nodeModulesAllowJsPackagePatternExportsTrailers.ts] ////

//// [index.js]
// esm format file
import * as cjsi from "inner/cjs/index.cjs";
import * as mjsi from "inner/mjs/index.mjs";
import * as typei from "inner/js/index.js";
cjsi;
mjsi;
typei;
//// [index.mjs]
// esm format file
import * as cjsi from "inner/cjs/index.cjs";
import * as mjsi from "inner/mjs/index.mjs";
import * as typei from "inner/js/index.js";
cjsi;
mjsi;
typei;
//// [index.cjs]
// cjs format file
import * as cjsi from "inner/cjs/index.cjs";
import * as mjsi from "inner/mjs/index.mjs";
import * as typei from "inner/js/index.js";
cjsi;
mjsi;
typei;
//// [index.d.ts]
// cjs format file
import * as cjs from "inner/cjs/index.cjs";
import * as mjs from "inner/mjs/index.mjs";
import * as type from "inner/js/index.js";
export { cjs };
export { mjs };
export { type };
//// [index.d.mts]
// esm format file
import * as cjs from "inner/cjs/index.cjs";
import * as mjs from "inner/mjs/index.mjs";
import * as type from "inner/js/index.js";
export { cjs };
export { mjs };
export { type };
//// [index.d.cts]
// cjs format file
import * as cjs from "inner/cjs/index.cjs";
import * as mjs from "inner/mjs/index.mjs";
import * as type from "inner/js/index.js";
export { cjs };
export { mjs };
export { type };
//// [package.json]
{
    "name": "package",
    "private": true,
    "type": "module"
}
//// [package.json]
{
    "name": "inner",
    "private": true,
    "exports": {
        "./cjs/*.cjs": "./*.cjs",
        "./mjs/*.mjs": "./*.mjs",
        "./js/*.js": "./*.js"
    }
}


//// [index.js]
// esm format file
import * as cjsi from "inner/cjs/index.cjs";
import * as mjsi from "inner/mjs/index.mjs";
import * as typei from "inner/js/index.js";
cjsi;
mjsi;
typei;
//// [index.mjs]
// esm format file
import * as cjsi from "inner/cjs/index.cjs";
import * as mjsi from "inner/mjs/index.mjs";
import * as typei from "inner/js/index.js";
cjsi;
mjsi;
typei;
//// [index.cjs]
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
// cjs format file
const cjsi = require("inner/cjs/index.cjs");
const mjsi = require("inner/mjs/index.mjs");
const typei = require("inner/js/index.js");
cjsi;
mjsi;
typei;


//// [index.d.ts]
export {};
//// [index.d.mts]
export {};
//// [index.d.cts]
export {};
