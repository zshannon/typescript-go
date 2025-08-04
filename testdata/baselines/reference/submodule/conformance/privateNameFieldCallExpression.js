//// [tests/cases/conformance/classes/members/privateNames/privateNameFieldCallExpression.ts] ////

//// [privateNameFieldCallExpression.ts]
class A {
    #fieldFunc = function() { this.x = 10; };
    #fieldFunc2 = function(a, ...b) {};
    x = 1;
    test() {
        this.#fieldFunc();
        this.#fieldFunc?.();
        const func = this.#fieldFunc;
        func();
        new this.#fieldFunc();

        const arr = [ 1, 2 ];
        this.#fieldFunc2(0, ...arr, 3);
        const b = new this.#fieldFunc2(0, ...arr, 3);
        const str = this.#fieldFunc2`head${1}middle${2}tail`;
        this.getInstance().#fieldFunc2`test${1}and${2}`;
    }
    getInstance() { return new A(); }
}


//// [privateNameFieldCallExpression.js]
class A {
    #fieldFunc = function () { this.x = 10; };
    #fieldFunc2 = function (a, ...b) { };
    x = 1;
    test() {
        var _a;
        this.#fieldFunc();
        (_a = this.#fieldFunc) === null || _a === void 0 ? void 0 : _a.call(this);
        const func = this.#fieldFunc;
        func();
        new this.#fieldFunc();
        const arr = [1, 2];
        this.#fieldFunc2(0, ...arr, 3);
        const b = new this.#fieldFunc2(0, ...arr, 3);
        const str = this.#fieldFunc2 `head${1}middle${2}tail`;
        this.getInstance().#fieldFunc2 `test${1}and${2}`;
    }
    getInstance() { return new A(); }
}
