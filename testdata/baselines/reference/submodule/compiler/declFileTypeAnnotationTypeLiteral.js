//// [tests/cases/compiler/declFileTypeAnnotationTypeLiteral.ts] ////

//// [declFileTypeAnnotationTypeLiteral.ts]
class c {
}
class g<T> {
}
module m {
    export class c {
    }
}

// Object literal with everything
var x: {
    // Call signatures
    (a: number): c;
    (a: string): g<string>;

    // Construct signatures
    new (a: number): c;
    new (a: string): m.c;

    // Indexers
    [n: number]: c;
    [n: string]: c;

    // Properties
    a: c;
    b: g<string>;

    // methods
    m1(): g<number>;
    m2(a: string, b?: number, ...c: c[]): string;
};


// Function type
var y: (a: string) => string;

// constructor type
var z: new (a: string) => m.c;

//// [declFileTypeAnnotationTypeLiteral.js]
class c {
}
class g {
}
var m;
(function (m) {
    class c {
    }
    m.c = c;
})(m || (m = {}));
// Object literal with everything
var x;
// Function type
var y;
// constructor type
var z;


//// [declFileTypeAnnotationTypeLiteral.d.ts]
declare class c {
}
declare class g<T> {
}
declare namespace m {
    class c {
    }
}
// Object literal with everything
declare var x: {
    // Call signatures
    (a: number): c;
    (a: string): g<string>;
    // Construct signatures
    new (a: number): c;
    new (a: string): m.c;
    // Indexers
    [n: number]: c;
    [n: string]: c;
    // Properties
    a: c;
    b: g<string>;
    // methods
    m1(): g<number>;
    m2(a: string, b?: number, ...c: c[]): string;
};
// Function type
declare var y: (a: string) => string;
// constructor type
declare var z: new (a: string) => m.c;
