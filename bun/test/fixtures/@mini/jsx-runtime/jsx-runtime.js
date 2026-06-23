// Mini JSX Runtime
export function jsx(type, props, key) {
    return { type, props, key: key ?? null };
}

export function jsxs(type, props, key) {
    return { type, props, key: key ?? null };
}

export function jsxDEV(type, props, key) {
    return { type, props, key: key ?? null };
}

export const Fragment = Symbol.for("mini.fragment");
