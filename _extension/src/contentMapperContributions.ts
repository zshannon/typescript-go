import type * as vscode from "vscode";

export interface ContentMapperManifest {
    readonly name: string;
    readonly version?: string;
    readonly exec: readonly string[];
    readonly cwd?: vscode.Uri;
    readonly compilerOptions?: readonly string[];
    readonly dynamicConfig?: boolean;
}

export interface ContentMapperContribution {
    readonly extensions: readonly string[];
    readonly inferredProject?: {
        readonly options?: Readonly<Record<string, unknown>>;
        readonly manifest: ContentMapperManifest;
    };
}

export interface SerializedContentMapperContribution {
    readonly contributorId: string;
    readonly extensions: readonly string[];
    readonly inferredProjectContribution?: {
        readonly options?: Readonly<Record<string, unknown>>;
        readonly manifest: {
            readonly name: string;
            readonly version?: string;
            readonly exec: readonly string[];
            readonly cwd?: string;
            readonly compilerOptions?: readonly string[];
            readonly dynamicConfig?: boolean;
        };
    };
}

export function serializeContentMapperContributions(
    registrations: ReadonlyMap<string, readonly ContentMapperContribution[]>,
): readonly SerializedContentMapperContribution[] {
    const result: SerializedContentMapperContribution[] = [];
    for (const [contributorId, contributions] of registrations) {
        contributions.forEach(contribution => {
            result.push({
                contributorId,
                extensions: [...contribution.extensions],
                inferredProjectContribution: contribution.inferredProject && {
                    options: contribution.inferredProject.options,
                    manifest: {
                        ...contribution.inferredProject.manifest,
                        exec: [...contribution.inferredProject.manifest.exec],
                        cwd: contribution.inferredProject.manifest.cwd?.fsPath,
                        compilerOptions: contribution.inferredProject.manifest.compilerOptions && [...contribution.inferredProject.manifest.compilerOptions],
                    },
                },
            });
        });
    }
    return result;
}

export function validateContentMapperRegistration(
    contributorId: string,
    contributions: readonly ContentMapperContribution[],
    registrations?: ReadonlyMap<string, readonly ContentMapperContribution[]>,
): void {
    if (!contributorId) {
        throw new TypeError("Content mapper contributor ID must not be empty.");
    }
    const claimedExtensions = new Set<string>();
    for (const registeredContributions of registrations?.values() ?? []) {
        for (const contribution of registeredContributions) {
            if (contribution.inferredProject) {
                contribution.extensions.forEach(extension => claimedExtensions.add(extension));
            }
        }
    }
    for (const contribution of contributions) {
        if (contribution.extensions.length === 0) {
            throw new TypeError("Content mapper contributions require non-empty extensions beginning with '.'.");
        }
        for (const extension of contribution.extensions) {
            if (!isValidContentMapperExtension(extension)) {
                throw new TypeError(`Content mapper contribution has invalid extension '${extension}'.`);
            }
        }
        const inferredProject = contribution.inferredProject;
        if (inferredProject?.options === null || Array.isArray(inferredProject?.options) || inferredProject?.options !== undefined && typeof inferredProject.options !== "object") {
            throw new TypeError("Content mapper contribution options must be an object.");
        }
        if (inferredProject && (!inferredProject.manifest.name || inferredProject.manifest.exec.length === 0)) {
            throw new TypeError("Content mapper contribution manifests require a name and non-empty exec.");
        }
        if (inferredProject?.manifest.cwd && inferredProject.manifest.cwd.scheme !== "file") {
            throw new TypeError("Content mapper contribution cwd must be a file URI.");
        }
        if (inferredProject) {
            for (const extension of contribution.extensions) {
                if (claimedExtensions.has(extension)) {
                    throw new TypeError(`Content mapper contributions both claim extension '${extension}'.`);
                }
                claimedExtensions.add(extension);
            }
        }
    }
}

export function documentMatchesContentMapperContributions(
    document: vscode.TextDocument,
    registrations: ReadonlyMap<string, readonly ContentMapperContribution[]>,
): boolean {
    for (const contributions of registrations.values()) {
        for (const contribution of contributions) {
            if (contribution.extensions.some(extension => document.uri.path.endsWith(extension))) {
                return true;
            }
        }
    }
    return false;
}

function isValidContentMapperExtension(extension: string): boolean {
    return /^\.[^./\\]+$/.test(extension) && !nativeTypeScriptExtensions.has(extension);
}

const nativeTypeScriptExtensions = new Set([
    ".cjs",
    ".cts",
    ".d.cts",
    ".d.mts",
    ".d.ts",
    ".js",
    ".json",
    ".jsx",
    ".mjs",
    ".mts",
    ".ts",
    ".tsx",
]);
