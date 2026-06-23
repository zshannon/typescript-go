/**
 * @module @flickfyi/tsgo
 * @description Blazing fast TypeScript type checker powered by TypeScript-Go
 *
 * The `fileName` parameter is a **virtual path** - it doesn't read from disk.
 * It's used for: error messages, file type detection (.ts vs .tsx), and import resolution.
 *
 * @example
 * ```typescript
 * import tsgo from '@flickfyi/tsgo';
 *
 * // Type check a string - fileName is virtual (not read from disk)
 * const result = tsgo.typecheck(
 *   `const x: number = 42;`,
 *   'input.ts'  // Virtual path: determines .ts/.tsx mode
 * );
 *
 * // JSX requires .tsx extension
 * const result = tsgo.typecheckWithOptions(
 *   `const App = () => <div>Hello</div>;`,
 *   'App.tsx',  // .tsx enables JSX
 *   { jsx: 'react-jsx', strict: true }
 * );
 *
 * // Multi-file: paths are used for import resolution
 * const result = tsgo.typecheckMultiple({
 *   'types.ts': 'export interface User { name: string; }',
 *   'main.ts': 'import { User } from "./types"; const u: User = { name: "Alice" };',
 * });
 * ```
 */

import {
    CString,
    dlopen,
    FFIType,
    type Pointer,
    ptr,
} from "bun:ffi";
import { existsSync } from "fs";
import {
    arch,
    platform,
} from "os";
import {
    dirname,
    join,
} from "path";

// ============================================================================
// Types
// ============================================================================

/**
 * TypeScript compiler options
 * @see https://www.typescriptlang.org/tsconfig
 */
export interface CompilerOptions {
    // Target & Module
    /** ECMAScript target version */
    target?: "ES5" | "ES6" | "ES2015" | "ES2016" | "ES2017" | "ES2018" | "ES2019" | "ES2020" | "ES2021" | "ES2022" | "ESNext";
    /** Module system to use */
    module?: "CommonJS" | "AMD" | "UMD" | "System" | "ES6" | "ES2015" | "ES2020" | "ES2022" | "ESNext" | "Node16" | "NodeNext" | "Preserve";
    /** Module resolution strategy */
    moduleResolution?: "Classic" | "Node" | "Node10" | "Node16" | "NodeNext" | "Bundler";
    /** Library files to include */
    lib?: string[];

    // JSX Options
    /** JSX emit mode */
    jsx?: "react" | "react-jsx" | "react-jsxdev" | "preserve";
    /** Module specifier for importing JSX factory functions (e.g., "react", "@emotion/react") */
    jsxImportSource?: string;
    /** JSX factory function (e.g., "React.createElement") */
    jsxFactory?: string;
    /** JSX fragment factory (e.g., "React.Fragment") */
    jsxFragmentFactory?: string;

    // Strict Type Checking
    /** Enable all strict type checking options */
    strict?: boolean;
    /** Enable strict null checks */
    strictNullChecks?: boolean;

    // Module Interop
    /** Enable ES module interop */
    esModuleInterop?: boolean;
    /** Allow importing JSON modules */
    resolveJsonModule?: boolean;
    /** Ensure each file can be safely transpiled */
    isolatedModules?: boolean;

    // Output
    /** Do not emit output files */
    noEmit?: boolean;
    /** Generate .d.ts declaration files */
    declaration?: boolean;

    // Other
    /** Allow JavaScript files */
    allowJs?: boolean;
    /** Skip type checking of declaration files */
    skipLibCheck?: boolean;
    /** Enforce consistent casing in file names */
    forceConsistentCasingInFileNames?: boolean;
}

/**
 * Diagnostic severity category
 */
export type DiagnosticCategory = "error" | "warning" | "suggestion" | "message";

/**
 * A diagnostic message from the type checker
 */
export interface Diagnostic {
    /** Error code (e.g., 2322 for type mismatch) */
    code: number;
    /** Diagnostic category */
    category: DiagnosticCategory;
    /** Human-readable error message */
    message: string;
    /** Source file path (for multi-file projects) */
    file?: string;
    /** 1-based line number */
    line?: number;
    /** 1-based column number */
    column?: number;
}

/**
 * Result of type checking operation
 */
export interface TypeCheckResult {
    /** Whether type checking passed with no errors */
    success: boolean;
    /** Array of diagnostic messages */
    diagnostics: Diagnostic[];
    /** Time taken in milliseconds */
    duration_ms: number;
}

// ============================================================================
// Binary Loading
// ============================================================================

/**
 * Get the path to the native binary for the current platform
 */
function getBinaryPath(): string {
    const currentPlatform = platform();
    const currentArch = arch();

    // Map Node.js platform/arch to our package names
    const platformMap: Record<string, string> = {
        darwin: "darwin",
        linux: "linux",
        win32: "win32",
    };

    const archMap: Record<string, string> = {
        arm64: "arm64",
        x64: "x64",
        x86_64: "x64",
    };

    const plat = platformMap[currentPlatform];
    const ar = archMap[currentArch];

    if (!plat || !ar) {
        throw new Error(
            `Unsupported platform: ${currentPlatform}-${currentArch}. ` +
                `Supported platforms: darwin-arm64, darwin-x64, linux-arm64, linux-x64, win32-x64`,
        );
    }

    const packageName = `@flickfyi/tsgo-${plat}-${ar}`;
    const packageNameNoScope = `tsgo-${plat}-${ar}`;
    const extension = currentPlatform === "win32" ? ".dll" : currentPlatform === "darwin" ? ".dylib" : ".so";
    const binaryName = `libtsgo${extension}`;

    // Try to find the binary in various locations
    const paths = [
        // Development mode - binary in package root (for local testing)
        join(import.meta.dir, "..", binaryName),
        // Development mode - binary in binaries subdirectory
        join(import.meta.dir, "..", "binaries", `${plat}-${ar}`, binaryName),
        // Platform package installed in this package's node_modules
        join(import.meta.dir, "..", "node_modules", packageName, binaryName),
        // Bun hoisted - platform package is sibling in @flickfyi scope (no scope prefix needed)
        join(import.meta.dir, "..", "..", packageNameNoScope, binaryName),
        // Platform package installed at project root node_modules
        join(process.cwd(), "node_modules", packageName, binaryName),
        // Hoisted in monorepo - check parent directories
        join(process.cwd(), "..", "node_modules", packageName, binaryName),
        join(process.cwd(), "..", "..", "node_modules", packageName, binaryName),
    ];

    for (const p of paths) {
        if (existsSync(p)) {
            return p;
        }
    }

    throw new Error(
        `Could not find tsgo binary for ${currentPlatform}-${currentArch}.\n` +
            `Tried paths:\n${paths.map(p => `  - ${p}`).join("\n")}\n\n` +
            `Please ensure the platform-specific package is installed:\n` +
            `  bun add @flickfyi/tsgo`,
    );
}

// ============================================================================
// FFI Setup
// ============================================================================

const binaryPath = getBinaryPath();

const lib = dlopen(binaryPath, {
    tsgo_typecheck: {
        args: [FFIType.ptr, FFIType.ptr, FFIType.ptr],
        returns: FFIType.ptr,
    },
    tsgo_typecheck_with_options: {
        args: [FFIType.ptr, FFIType.ptr, FFIType.ptr, FFIType.ptr],
        returns: FFIType.ptr,
    },
    tsgo_typecheck_multiple: {
        args: [FFIType.ptr, FFIType.ptr, FFIType.ptr],
        returns: FFIType.ptr,
    },
    tsgo_version: {
        args: [],
        returns: FFIType.ptr,
    },
    tsgo_free_string: {
        args: [FFIType.ptr],
        returns: FFIType.void,
    },
});

// Get the wrapper directory for finding node_modules
// This should be the package root or the current working directory
const wrapperDir = process.cwd();

// ============================================================================
// Helper Functions
// ============================================================================

/** Convert string to null-terminated C string buffer */
function toCString(str: string): Buffer {
    return Buffer.from(str + "\0", "utf8");
}

/** Read C string from pointer and free it */
function readAndFreeString(pointer: Pointer | null): string {
    if (!pointer) {
        return "";
    }
    const cstr = new CString(pointer);
    const result = cstr.toString();
    lib.symbols.tsgo_free_string(pointer);
    return result;
}

function parseResult(resultPtr: Pointer | null): TypeCheckResult {
    if (!resultPtr) {
        return {
            success: false,
            diagnostics: [{ code: 0, category: "error", message: "FFI call returned null" }],
            duration_ms: 0,
        };
    }

    const resultStr = readAndFreeString(resultPtr);

    try {
        return JSON.parse(resultStr);
    }
    catch {
        return {
            success: false,
            diagnostics: [{ code: 0, category: "error", message: `Failed to parse result: ${resultStr}` }],
            duration_ms: 0,
        };
    }
}

// ============================================================================
// Public API
// ============================================================================

/**
 * Get the version of the tsgo library
 *
 * @returns Version string (e.g., "1.0.0")
 *
 * @example
 * ```typescript
 * console.log(tsgo.version()); // "1.0.0"
 * ```
 */
function version(): string {
    const resultPtr = lib.symbols.tsgo_version();
    if (!resultPtr) return "unknown";
    const cstr = new CString(resultPtr);
    return cstr.toString();
}

/**
 * Type check a single TypeScript/TSX file
 *
 * Uses sensible defaults optimized for modern TypeScript development:
 * - strict: true
 * - jsx: react-jsx
 * - module: ESNext
 * - moduleResolution: Bundler
 * - target: ES2022
 *
 * @param code - The TypeScript source code to check
 * @param fileName - Virtual path (not read from disk). Use .tsx for JSX files.
 * @param projectDir - Directory for resolving node_modules (defaults to cwd)
 * @returns Type checking result with diagnostics
 *
 * @example
 * ```typescript
 * // Basic TypeScript - .ts extension
 * const result = tsgo.typecheck(`const x: number = "wrong";`, 'file.ts');
 *
 * // React component - .tsx extension enables JSX
 * const result = tsgo.typecheck(`
 *   import React from 'react';
 *   const App: React.FC = () => <div>Hello</div>;
 * `, 'App.tsx');
 * ```
 */
function typecheck(code: string, fileName: string, projectDir?: string): TypeCheckResult {
    const codeBuffer = toCString(code);
    const fileNameBuffer = toCString(fileName);
    const projectDirBuffer = toCString(projectDir || wrapperDir);

    const resultPtr = lib.symbols.tsgo_typecheck(
        ptr(codeBuffer),
        ptr(fileNameBuffer),
        ptr(projectDirBuffer),
    );
    return parseResult(resultPtr);
}

/**
 * Type check a single file with custom compiler options
 *
 * @param code - The TypeScript source code to check
 * @param fileName - Virtual file path (used for error messages and JSX detection)
 * @param options - TypeScript compiler options
 * @returns Type checking result with diagnostics
 *
 * @example
 * ```typescript
 * // Custom JSX runtime (e.g., Emotion, Preact, custom)
 * const result = tsgo.typecheckWithOptions(code, 'App.tsx', {
 *   jsx: 'react-jsx',
 *   jsxImportSource: '@emotion/react',
 *   strict: true,
 * });
 *
 * // Legacy React
 * const result = tsgo.typecheckWithOptions(code, 'App.tsx', {
 *   jsx: 'react',
 *   jsxFactory: 'React.createElement',
 *   jsxFragmentFactory: 'React.Fragment',
 * });
 * ```
 */
function typecheckWithOptions(
    code: string,
    fileName: string,
    options: CompilerOptions,
    projectDir?: string,
): TypeCheckResult {
    const codeBuffer = toCString(code);
    const fileNameBuffer = toCString(fileName);
    const optionsBuffer = toCString(JSON.stringify(options));
    const projectDirBuffer = toCString(projectDir || wrapperDir);

    const resultPtr = lib.symbols.tsgo_typecheck_with_options(
        ptr(codeBuffer),
        ptr(fileNameBuffer),
        ptr(optionsBuffer),
        ptr(projectDirBuffer),
    );
    return parseResult(resultPtr);
}

/**
 * Type check multiple files as a project
 *
 * Files can import from each other using relative paths.
 * The type checker resolves imports between virtual files.
 *
 * @param files - Map of file paths to source code
 * @param options - TypeScript compiler options (optional)
 * @returns Type checking result with diagnostics for all files
 *
 * @example
 * ```typescript
 * const result = tsgo.typecheckMultiple({
 *   'types.ts': `
 *     export interface User { id: number; name: string; }
 *   `,
 *   'utils.ts': `
 *     import { User } from './types';
 *     export const greet = (u: User) => \`Hello, \${u.name}!\`;
 *   `,
 *   'main.ts': `
 *     import { User } from './types';
 *     import { greet } from './utils';
 *     const user: User = { id: 1, name: 'Alice' };
 *   `,
 * }, { strict: true });
 * ```
 */
function typecheckMultiple(
    files: Record<string, string>,
    options?: CompilerOptions,
    projectDir?: string,
): TypeCheckResult {
    const filesBuffer = toCString(JSON.stringify(files));
    const optionsBuffer = toCString(options ? JSON.stringify(options) : "");
    const projectDirBuffer = toCString(projectDir || wrapperDir);

    const resultPtr = lib.symbols.tsgo_typecheck_multiple(
        ptr(filesBuffer),
        ptr(optionsBuffer),
        ptr(projectDirBuffer),
    );
    return parseResult(resultPtr);
}

// ============================================================================
// Default Export
// ============================================================================

const tsgo = {
    version,
    typecheck,
    typecheckWithOptions,
    typecheckMultiple,
};

export default tsgo;
export { typecheck, typecheckMultiple, typecheckWithOptions, version };
