#!/usr/bin/env bun
/**
 * Sync optionalDependencies versions to match the main package version.
 * Run this after `npm version` to keep platform package versions in sync.
 */

import { join } from "path";

const pkgPath = join(import.meta.dir, "..", "package.json");
const pkg = await Bun.file(pkgPath).json();

const platforms = [
    "darwin-arm64",
    "darwin-x64",
    "linux-arm64",
    "linux-x64",
    "win32-x64",
];

for (const platform of platforms) {
    pkg.optionalDependencies[`@flickfyi/tsgo-${platform}`] = pkg.version;
}

await Bun.write(pkgPath, JSON.stringify(pkg, null, 2) + "\n");

console.log(`Synced optionalDependencies to version ${pkg.version}`);
