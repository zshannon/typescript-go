//// [tests/cases/compiler/destructureOptionalParameter.ts] ////

//// [destructureOptionalParameter.ts]
declare function f1({ a, b }?: { a: number, b: string }): void;

function f2({ a, b }: { a: number, b: number } = { a: 0, b: 0 }) {
    a;
    b;
}

// Repro from #8681

interface Type { t: void }
interface QueryMetadata { q: void }

interface QueryMetadataFactory {
    (selector: Type | string, {descendants, read}?: {
        descendants?: boolean;
        read?: any;
    }): ParameterDecorator;
    new (selector: Type | string, {descendants, read}?: {
        descendants?: boolean;
        read?: any;
    }): QueryMetadata;
}


//// [destructureOptionalParameter.js]
function f2({ a, b } = { a: 0, b: 0 }) {
    a;
    b;
}


//// [destructureOptionalParameter.d.ts]
declare function f1({ a, b }?: {
    a: number;
    b: string;
}): void;
declare function f2({ a, b }?: {
    a: number;
    b: number;
}): void;
// Repro from #8681
interface Type {
    t: void;
}
interface QueryMetadata {
    q: void;
}
interface QueryMetadataFactory {
    (selector: Type | string, { descendants, read }?: {
        descendants?: boolean;
        read?: any;
    }): ParameterDecorator;
    new (selector: Type | string, { descendants, read }?: {
        descendants?: boolean;
        read?: any;
    }): QueryMetadata;
}
