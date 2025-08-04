//// [tests/cases/conformance/classes/members/privateNames/privateNameStaticFieldCallExpression.ts] ////

//// [privateNameStaticFieldCallExpression.ts]
class A {
    static #fieldFunc = function () { this.x = 10; };
    static #fieldFunc2 = function (a, ...b) {};
    x = 1;
    test() {
        A.#fieldFunc();
        A.#fieldFunc?.();
        const func = A.#fieldFunc;
        func();
        new A.#fieldFunc();

        const arr = [ 1, 2 ];
        A.#fieldFunc2(0, ...arr, 3);
        const b = new A.#fieldFunc2(0, ...arr, 3);
        const str = A.#fieldFunc2`head${1}middle${2}tail`;
        this.getClass().#fieldFunc2`test${1}and${2}`;
    }
    getClass() { return A; }
}


//// [privateNameStaticFieldCallExpression.js]
class A {
    static #fieldFunc = function () { this.x = 10; };
    static #fieldFunc2 = function (a, ...b) { };
    x = 1;
    test() {
        var _a;
        A.#fieldFunc();
        (_a = A.#fieldFunc) === null || _a === void 0 ? void 0 : _a.call(A);
        const func = A.#fieldFunc;
        func();
        new A.#fieldFunc();
        const arr = [1, 2];
        A.#fieldFunc2(0, ...arr, 3);
        const b = new A.#fieldFunc2(0, ...arr, 3);
        const str = A.#fieldFunc2 `head${1}middle${2}tail`;
        this.getClass().#fieldFunc2 `test${1}and${2}`;
    }
    getClass() { return A; }
}
