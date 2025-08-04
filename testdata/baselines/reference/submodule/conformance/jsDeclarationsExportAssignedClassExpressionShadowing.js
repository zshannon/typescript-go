//// [tests/cases/conformance/jsdoc/declarations/jsDeclarationsExportAssignedClassExpressionShadowing.ts] ////

//// [index.js]
class A {
    member = new Q();
}
class Q {
    x = 42;
}
module.exports = class Q {
    constructor() {
        this.x = new A();
    }
}
module.exports.Another = Q;


//// [index.js]
"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
class A {
    member = new Q();
}
class Q {
    x = 42;
}
export = class Q {
    constructor() {
        this.x = new A();
    }
};
module.exports = class Q {
    constructor() {
        this.x = new A();
    }
};
export var Another = Q;
module.exports.Another = Q;


//// [index.d.ts]
declare class A {
    member: Q;
}
declare class Q {
    x: number;
}
declare const _default: {
    new (): {
        x: A;
    };
};
export = _default;
export var Another = Q;
