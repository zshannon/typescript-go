//// [tests/cases/conformance/classes/members/privateNames/privateNameFieldAssignment.ts] ////

//// [privateNameFieldAssignment.ts]
class A {
    #field = 0;
    constructor() {
        this.#field = 1;
        this.#field += 2;
        this.#field -= 3;
        this.#field /= 4;
        this.#field *= 5;
        this.#field **= 6;
        this.#field %= 7;
        this.#field <<= 8;
        this.#field >>= 9;
        this.#field >>>= 10;
        this.#field &= 11;
        this.#field |= 12;
        this.#field ^= 13;
        A.getInstance().#field = 1;
        A.getInstance().#field += 2;
        A.getInstance().#field -= 3;
        A.getInstance().#field /= 4;
        A.getInstance().#field *= 5;
        A.getInstance().#field **= 6;
        A.getInstance().#field %= 7;
        A.getInstance().#field <<= 8;
        A.getInstance().#field >>= 9;
        A.getInstance().#field >>>= 10;
        A.getInstance().#field &= 11;
        A.getInstance().#field |= 12;
        A.getInstance().#field ^= 13;
    }
    static getInstance() {
        return new A();
    }
}


//// [privateNameFieldAssignment.js]
class A {
    #field = 0;
    constructor() {
        var _a, _b;
        this.#field = 1;
        this.#field += 2;
        this.#field -= 3;
        this.#field /= 4;
        this.#field *= 5;
        (_a = this).#field = Math.pow(_a.#field, 6);
        this.#field %= 7;
        this.#field <<= 8;
        this.#field >>= 9;
        this.#field >>>= 10;
        this.#field &= 11;
        this.#field |= 12;
        this.#field ^= 13;
        A.getInstance().#field = 1;
        A.getInstance().#field += 2;
        A.getInstance().#field -= 3;
        A.getInstance().#field /= 4;
        A.getInstance().#field *= 5;
        (_b = A.getInstance()).#field = Math.pow(_b.#field, 6);
        A.getInstance().#field %= 7;
        A.getInstance().#field <<= 8;
        A.getInstance().#field >>= 9;
        A.getInstance().#field >>>= 10;
        A.getInstance().#field &= 11;
        A.getInstance().#field |= 12;
        A.getInstance().#field ^= 13;
    }
    static getInstance() {
        return new A();
    }
}
