currentDirectory::/home/src/workspaces/project
useCaseSensitiveFileNames::true
Input::
//// [/home/src/workspaces/project/node_modules/@types/react/index.d.ts] *new* 
export {};
declare global {
    namespace JSX {
        interface Element {}
        interface IntrinsicElements {
            div: {
                propA?: boolean;
            };
        }
    }
}
//// [/home/src/workspaces/project/node_modules/react/jsx-runtime.js] *new* 
export {}
//// [/home/src/workspaces/project/src/index.tsx] *new* 
export const App = () => <div propA={true}></div>;
//// [/home/src/workspaces/project/tsconfig.json] *new* 
{ 
    "compilerOptions": { 
        "module": "commonjs",
        "jsx": "react-jsx", 
        "incremental": true, 
        "jsxImportSource": "react" 
    } 
}

tsgo --strict
ExitStatus:: DiagnosticsPresent_OutputsGenerated
Output::
[96msrc/index.tsx[0m:[93m1[0m:[93m26[0m - [91merror[0m[90m TS7016: [0mCould not find a declaration file for module 'react/jsx-runtime'. '/home/src/workspaces/project/node_modules/react/jsx-runtime.js' implicitly has an 'any' type.

[7m1[0m export const App = () => <div propA={true}></div>;
[7m [0m [91m                         ~~~~~~~~~~~~~~~~~~~~~~~~[0m


Found 1 error in src/index.tsx[90m:1[0m

//// [/home/src/tslibs/TS/Lib/lib.d.ts] *Lib*
/// <reference no-default-lib="true"/>
interface Boolean {}
interface Function {}
interface CallableFunction {}
interface NewableFunction {}
interface IArguments {}
interface Number { toExponential: any; }
interface Object {}
interface RegExp {}
interface String { charAt: any; }
interface Array<T> { length: number; [n: number]: T; }
interface ReadonlyArray<T> {}
interface SymbolConstructor {
    (desc?: string | number): symbol;
    for(name: string): symbol;
    readonly toStringTag: symbol;
}
declare var Symbol: SymbolConstructor;
interface Symbol {
    readonly [Symbol.toStringTag]: string;
}
declare const console: { log(msg: any): void; };
//// [/home/src/workspaces/project/src/index.js] *new* 
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.App = void 0;
const jsx_runtime_1 = require("react/jsx-runtime");
const App = () => jsx_runtime_1.jsx("div", { propA: true });
exports.App = App;

//// [/home/src/workspaces/project/tsconfig.tsbuildinfo] *new* 
{"version":"FakeTSVersion","fileNames":["../../tslibs/TS/Lib/lib.d.ts","./src/index.tsx","./node_modules/@types/react/index.d.ts"],"fileInfos":[{"version":"8859c12c614ce56ba9a18e58384a198f-/// <reference no-default-lib=\"true\"/>\ninterface Boolean {}\ninterface Function {}\ninterface CallableFunction {}\ninterface NewableFunction {}\ninterface IArguments {}\ninterface Number { toExponential: any; }\ninterface Object {}\ninterface RegExp {}\ninterface String { charAt: any; }\ninterface Array<T> { length: number; [n: number]: T; }\ninterface ReadonlyArray<T> {}\ninterface SymbolConstructor {\n    (desc?: string | number): symbol;\n    for(name: string): symbol;\n    readonly toStringTag: symbol;\n}\ndeclare var Symbol: SymbolConstructor;\ninterface Symbol {\n    readonly [Symbol.toStringTag]: string;\n}\ndeclare const console: { log(msg: any): void; };","affectsGlobalScope":true,"impliedNodeFormat":1},"0c3575e38f9aabc971cea9c73a211979-export const App = () => <div propA={true}></div>;",{"version":"a6afc3c631ce6ad7bc83db84287b52ea-export {};\ndeclare global {\n    namespace JSX {\n        interface Element {}\n        interface IntrinsicElements {\n            div: {\n                propA?: boolean;\n            };\n        }\n    }\n}","affectsGlobalScope":true,"impliedNodeFormat":1}],"options":{"jsx":4,"jsxImportSource":"react","module":1,"strict":true},"semanticDiagnosticsPerFile":[[2,[{"pos":25,"end":49,"code":7016,"category":1,"message":"Could not find a declaration file for module 'react/jsx-runtime'. '/home/src/workspaces/project/node_modules/react/jsx-runtime.js' implicitly has an 'any' type."}]]]}
//// [/home/src/workspaces/project/tsconfig.tsbuildinfo.readable.baseline.txt] *new* 
{
  "version": "FakeTSVersion",
  "fileNames": [
    "../../tslibs/TS/Lib/lib.d.ts",
    "./src/index.tsx",
    "./node_modules/@types/react/index.d.ts"
  ],
  "fileInfos": [
    {
      "fileName": "../../tslibs/TS/Lib/lib.d.ts",
      "version": "8859c12c614ce56ba9a18e58384a198f-/// <reference no-default-lib=\"true\"/>\ninterface Boolean {}\ninterface Function {}\ninterface CallableFunction {}\ninterface NewableFunction {}\ninterface IArguments {}\ninterface Number { toExponential: any; }\ninterface Object {}\ninterface RegExp {}\ninterface String { charAt: any; }\ninterface Array<T> { length: number; [n: number]: T; }\ninterface ReadonlyArray<T> {}\ninterface SymbolConstructor {\n    (desc?: string | number): symbol;\n    for(name: string): symbol;\n    readonly toStringTag: symbol;\n}\ndeclare var Symbol: SymbolConstructor;\ninterface Symbol {\n    readonly [Symbol.toStringTag]: string;\n}\ndeclare const console: { log(msg: any): void; };",
      "signature": "8859c12c614ce56ba9a18e58384a198f-/// <reference no-default-lib=\"true\"/>\ninterface Boolean {}\ninterface Function {}\ninterface CallableFunction {}\ninterface NewableFunction {}\ninterface IArguments {}\ninterface Number { toExponential: any; }\ninterface Object {}\ninterface RegExp {}\ninterface String { charAt: any; }\ninterface Array<T> { length: number; [n: number]: T; }\ninterface ReadonlyArray<T> {}\ninterface SymbolConstructor {\n    (desc?: string | number): symbol;\n    for(name: string): symbol;\n    readonly toStringTag: symbol;\n}\ndeclare var Symbol: SymbolConstructor;\ninterface Symbol {\n    readonly [Symbol.toStringTag]: string;\n}\ndeclare const console: { log(msg: any): void; };",
      "affectsGlobalScope": true,
      "impliedNodeFormat": "CommonJS",
      "original": {
        "version": "8859c12c614ce56ba9a18e58384a198f-/// <reference no-default-lib=\"true\"/>\ninterface Boolean {}\ninterface Function {}\ninterface CallableFunction {}\ninterface NewableFunction {}\ninterface IArguments {}\ninterface Number { toExponential: any; }\ninterface Object {}\ninterface RegExp {}\ninterface String { charAt: any; }\ninterface Array<T> { length: number; [n: number]: T; }\ninterface ReadonlyArray<T> {}\ninterface SymbolConstructor {\n    (desc?: string | number): symbol;\n    for(name: string): symbol;\n    readonly toStringTag: symbol;\n}\ndeclare var Symbol: SymbolConstructor;\ninterface Symbol {\n    readonly [Symbol.toStringTag]: string;\n}\ndeclare const console: { log(msg: any): void; };",
        "affectsGlobalScope": true,
        "impliedNodeFormat": 1
      }
    },
    {
      "fileName": "./src/index.tsx",
      "version": "0c3575e38f9aabc971cea9c73a211979-export const App = () => <div propA={true}></div>;",
      "signature": "0c3575e38f9aabc971cea9c73a211979-export const App = () => <div propA={true}></div>;",
      "impliedNodeFormat": "CommonJS"
    },
    {
      "fileName": "./node_modules/@types/react/index.d.ts",
      "version": "a6afc3c631ce6ad7bc83db84287b52ea-export {};\ndeclare global {\n    namespace JSX {\n        interface Element {}\n        interface IntrinsicElements {\n            div: {\n                propA?: boolean;\n            };\n        }\n    }\n}",
      "signature": "a6afc3c631ce6ad7bc83db84287b52ea-export {};\ndeclare global {\n    namespace JSX {\n        interface Element {}\n        interface IntrinsicElements {\n            div: {\n                propA?: boolean;\n            };\n        }\n    }\n}",
      "affectsGlobalScope": true,
      "impliedNodeFormat": "CommonJS",
      "original": {
        "version": "a6afc3c631ce6ad7bc83db84287b52ea-export {};\ndeclare global {\n    namespace JSX {\n        interface Element {}\n        interface IntrinsicElements {\n            div: {\n                propA?: boolean;\n            };\n        }\n    }\n}",
        "affectsGlobalScope": true,
        "impliedNodeFormat": 1
      }
    }
  ],
  "options": {
    "jsx": 4,
    "jsxImportSource": "react",
    "module": 1,
    "strict": true
  },
  "semanticDiagnosticsPerFile": [
    [
      "./src/index.tsx",
      [
        {
          "pos": 25,
          "end": 49,
          "code": 7016,
          "category": 1,
          "message": "Could not find a declaration file for module 'react/jsx-runtime'. '/home/src/workspaces/project/node_modules/react/jsx-runtime.js' implicitly has an 'any' type."
        }
      ]
    ]
  ],
  "size": 1632
}

SemanticDiagnostics::
*refresh*    /home/src/tslibs/TS/Lib/lib.d.ts
*refresh*    /home/src/workspaces/project/src/index.tsx
*refresh*    /home/src/workspaces/project/node_modules/@types/react/index.d.ts
Signatures::
