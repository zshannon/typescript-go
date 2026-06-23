#!/usr/bin/env bun
/**
 * Generate platform-specific npm packages for tsgo binaries
 *
 * This creates separate npm packages for each platform:
 *   @flickfyi/tsgo-darwin-arm64
 *   @flickfyi/tsgo-darwin-x64
 *   @flickfyi/tsgo-linux-arm64
 *   @flickfyi/tsgo-linux-x64
 *   @flickfyi/tsgo-win32-x64
 *
 * These are installed as optionalDependencies, so only the matching platform is downloaded.
 */

import {
    copyFileSync,
    existsSync,
    mkdirSync,
    writeFileSync,
} from "fs";
import { join } from "path";

// Read version from main package.json
const mainPkg = await Bun.file(join(import.meta.dir, "..", "package.json")).json();
const VERSION = mainPkg.version;
const SCOPE = "@flickfyi";

interface PlatformConfig {
    name: string;
    os: string;
    cpu: string;
    ext: string;
}

const platforms: PlatformConfig[] = [
    { name: "darwin-arm64", os: "darwin", cpu: "arm64", ext: "dylib" },
    { name: "darwin-x64", os: "darwin", cpu: "x64", ext: "dylib" },
    { name: "linux-arm64", os: "linux", cpu: "arm64", ext: "so" },
    { name: "linux-x64", os: "linux", cpu: "x64", ext: "so" },
    { name: "win32-x64", os: "win32", cpu: "x64", ext: "dll" },
];

const bunDir = join(import.meta.dir, "..");
const outputDir = join(bunDir, "platforms");
const binariesDir = join(bunDir, "binaries");

console.log("Generating platform-specific packages...");
console.log(`Version: ${VERSION}`);
console.log(`Output: ${outputDir}`);
console.log("");

for (const platform of platforms) {
    const platformDir = join(outputDir, platform.name);
    const packageName = `${SCOPE}/tsgo-${platform.name}`;
    const binaryName = `libtsgo.${platform.ext}`;

    console.log(`Creating ${packageName}...`);

    // Create directory
    mkdirSync(platformDir, { recursive: true });

    // Create package.json
    const packageJson = {
        name: packageName,
        version: VERSION,
        description: `tsgo native binary for ${platform.name}`,
        license: "Apache-2.0",
        repository: {
            type: "git",
            url: "https://github.com/zshannon/typescript-go",
            directory: "bun",
        },
        os: [platform.os],
        cpu: [platform.cpu],
        main: binaryName,
        files: [binaryName],
        preferUnplugged: true,
    };

    writeFileSync(
        join(platformDir, "package.json"),
        JSON.stringify(packageJson, null, 2) + "\n",
    );

    // Create README
    const readme = `# ${packageName}

Platform-specific binary for [@flickfyi/tsgo](https://www.npmjs.com/package/@flickfyi/tsgo).

This package contains the native binary for ${platform.os}/${platform.cpu}.

**Do not install this package directly.** Install \`@flickfyi/tsgo\` instead:

\`\`\`bash
bun add @flickfyi/tsgo
\`\`\`

The correct platform package will be installed automatically as an optional dependency.
`;

    writeFileSync(join(platformDir, "README.md"), readme);

    // Copy binary if it exists
    const sourceBinary = join(binariesDir, platform.name, binaryName);
    const destBinary = join(platformDir, binaryName);

    if (existsSync(sourceBinary)) {
        copyFileSync(sourceBinary, destBinary);
        const size = Bun.file(destBinary).size;
        console.log(`  Copied ${binaryName} (${(size / 1024 / 1024).toFixed(1)}MB)`);
    }
    else {
        console.log(`  [WARN] Binary not found: ${sourceBinary}`);
    }
}

console.log("");
console.log("Done! Platform packages created in:");
console.log(`  ${outputDir}`);
console.log("");
console.log("To publish all packages:");
console.log(`  for p in ${platforms.map(p => p.name).join(" ")}; do`);
console.log(`    cd ${outputDir}/$p && npm publish --access public`);
console.log(`  done`);
