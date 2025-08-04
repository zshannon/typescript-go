//// [tests/cases/compiler/verbatim-declarations-parameters.ts] ////

//// [verbatim-declarations-parameters.ts]
type Map = {} & { [P in string]: any }
type MapOrUndefined = Map | undefined | "dummy"
export class Foo {
  constructor(
    // Type node is accurate, preserve
    public reuseTypeNode?: Map | undefined,
    public reuseTypeNode2?: Exclude<MapOrUndefined, "dummy">,
    // Resolve type node, requires adding | undefined
    public resolveType?: Map,
  ) { }
}

export function foo1(
    // Type node is accurate, preserve
    reuseTypeNode: Map | undefined = {},
    reuseTypeNode2: Exclude<MapOrUndefined, "dummy">  = {},
    // Resolve type node, requires adding | undefined
    resolveType: Map = {}, 
    requiredParam: number) {

}


//// [verbatim-declarations-parameters.js]
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.Foo = void 0;
exports.foo1 = foo1;
class Foo {
    reuseTypeNode;
    reuseTypeNode2;
    resolveType;
    constructor(
    // Type node is accurate, preserve
    reuseTypeNode, reuseTypeNode2, 
    // Resolve type node, requires adding | undefined
    resolveType) {
        this.reuseTypeNode = reuseTypeNode;
        this.reuseTypeNode2 = reuseTypeNode2;
        this.resolveType = resolveType;
    }
}
exports.Foo = Foo;
function foo1(
// Type node is accurate, preserve
reuseTypeNode = {}, reuseTypeNode2 = {}, 
// Resolve type node, requires adding | undefined
resolveType = {}, requiredParam) {
}


//// [verbatim-declarations-parameters.d.ts]
type Map = {} & {
    [P in string]: any;
};
type MapOrUndefined = Map | undefined | "dummy";
export declare class Foo {
    reuseTypeNode?: Map | undefined;
    reuseTypeNode2?: Exclude<MapOrUndefined, "dummy">;
    resolveType?: {
        [x: string]: any;
    } | undefined;
    constructor(
    // Type node is accurate, preserve
    reuseTypeNode?: Map | undefined, reuseTypeNode2?: Exclude<MapOrUndefined, "dummy">, 
    // Resolve type node, requires adding | undefined
    resolveType?: {
        [x: string]: any;
    } | undefined);
}
export declare function foo1(
// Type node is accurate, preserve
reuseTypeNode: Map | undefined, reuseTypeNode2: Exclude<MapOrUndefined, "dummy">, 
// Resolve type node, requires adding | undefined
resolveType: Map, requiredParam: number): void;
export {};
