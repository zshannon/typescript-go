//// [tests/cases/conformance/expressions/optionalChaining/callChain/callChain.ts] ////

//// [callChain.ts]
declare const o1: undefined | ((...args: any[]) => number);
o1?.();
o1?.(1);
o1?.(...[1, 2]);
o1?.(1, ...[2, 3], 4);

declare const o2: undefined | { b: (...args: any[]) => number };
o2?.b();
o2?.b(1);
o2?.b(...[1, 2]);
o2?.b(1, ...[2, 3], 4);
o2?.["b"]();
o2?.["b"](1);
o2?.["b"](...[1, 2]);
o2?.["b"](1, ...[2, 3], 4);

declare const o3: { b: ((...args: any[]) => { c: string }) | undefined };
o3.b?.().c;
o3.b?.(1).c;
o3.b?.(...[1, 2]).c;
o3.b?.(1, ...[2, 3], 4).c;
o3.b?.()["c"];
o3.b?.(1)["c"];
o3.b?.(...[1, 2])["c"];
o3.b?.(1, ...[2, 3], 4)["c"];
o3["b"]?.().c;
o3["b"]?.(1).c;
o3["b"]?.(...[1, 2]).c;
o3["b"]?.(1, ...[2, 3], 4).c;

declare const o4: undefined | (<T>(f: (a: T) => T) => T);
declare function incr(x: number): number;
const v: number | undefined = o4?.(incr);

// GH#33744
declare const o5: <T>() => undefined | (() => void);
o5<number>()?.();

// GH#36031
o2?.b()!.toString;
o2?.b()!.toString!;

//// [callChain.js]
var _a, _b, _c, _d, _e, _f, _g, _h, _j, _k, _l, _m, _o;
o1 === null || o1 === void 0 ? void 0 : o1();
o1 === null || o1 === void 0 ? void 0 : o1(1);
o1 === null || o1 === void 0 ? void 0 : o1(...[1, 2]);
o1 === null || o1 === void 0 ? void 0 : o1(1, ...[2, 3], 4);
o2 === null || o2 === void 0 ? void 0 : o2.b();
o2 === null || o2 === void 0 ? void 0 : o2.b(1);
o2 === null || o2 === void 0 ? void 0 : o2.b(...[1, 2]);
o2 === null || o2 === void 0 ? void 0 : o2.b(1, ...[2, 3], 4);
o2 === null || o2 === void 0 ? void 0 : o2["b"]();
o2 === null || o2 === void 0 ? void 0 : o2["b"](1);
o2 === null || o2 === void 0 ? void 0 : o2["b"](...[1, 2]);
o2 === null || o2 === void 0 ? void 0 : o2["b"](1, ...[2, 3], 4);
(_a = o3.b) === null || _a === void 0 ? void 0 : _a.call(o3).c;
(_b = o3.b) === null || _b === void 0 ? void 0 : _b.call(o3, 1).c;
(_c = o3.b) === null || _c === void 0 ? void 0 : _c.call(o3, ...[1, 2]).c;
(_d = o3.b) === null || _d === void 0 ? void 0 : _d.call(o3, 1, ...[2, 3], 4).c;
(_e = o3.b) === null || _e === void 0 ? void 0 : _e.call(o3)["c"];
(_f = o3.b) === null || _f === void 0 ? void 0 : _f.call(o3, 1)["c"];
(_g = o3.b) === null || _g === void 0 ? void 0 : _g.call(o3, ...[1, 2])["c"];
(_h = o3.b) === null || _h === void 0 ? void 0 : _h.call(o3, 1, ...[2, 3], 4)["c"];
(_j = o3["b"]) === null || _j === void 0 ? void 0 : _j.call(o3).c;
(_k = o3["b"]) === null || _k === void 0 ? void 0 : _k.call(o3, 1).c;
(_l = o3["b"]) === null || _l === void 0 ? void 0 : _l.call(o3, ...[1, 2]).c;
(_m = o3["b"]) === null || _m === void 0 ? void 0 : _m.call(o3, 1, ...[2, 3], 4).c;
const v = o4 === null || o4 === void 0 ? void 0 : o4(incr);
(_o = o5()) === null || _o === void 0 ? void 0 : _o();
// GH#36031
o2 === null || o2 === void 0 ? void 0 : o2.b().toString;
o2 === null || o2 === void 0 ? void 0 : o2.b().toString;
