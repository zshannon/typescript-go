#!/usr/bin/env bun
/**
 * Generate platform-specific npm packages for tsgo binaries
 *
 * Creates:
 *   @tsgo/darwin-arm64
 *   @tsgo/darwin-x64
 *   @tsgo/linux-arm64
 *   @tsgo/linux-x64
 *   @tsgo/win32-x64
 */

import { mkdirSync, writeFileSync, existsSync, copyFileSync } from "fs";
import { join } from "path";

const VERSION = "0.1.0";

interface PlatformConfig {
  name: string;
  os: string;
  cpu: string;
  binaryName: string;
}

const platforms: PlatformConfig[] = [
  { name: "darwin-arm64", os: "darwin", cpu: "arm64", binaryName: "libtsgo.dylib" },
  { name: "darwin-x64", os: "darwin", cpu: "x64", binaryName: "libtsgo.dylib" },
  { name: "linux-arm64", os: "linux", cpu: "arm64", binaryName: "libtsgo.so" },
  { name: "linux-x64", os: "linux", cpu: "x64", binaryName: "libtsgo.so" },
  { name: "win32-x64", os: "win32", cpu: "x64", binaryName: "libtsgo.dll" },
];

const scriptDir = import.meta.dir;
const packageDir = join(scriptDir, "..");
const outputDir = join(packageDir, "platforms");
const binariesDir = join(packageDir, "binaries");

console.log("Generating platform-specific packages...");
console.log(`Output: ${outputDir}`);
console.log("");

for (const platform of platforms) {
  const platformDir = join(outputDir, platform.name);
  const packageName = `@tsgo/${platform.name}`;

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
      url: "https://github.com/anthropics/typescript-go",
    },
    os: [platform.os],
    cpu: [platform.cpu],
    files: [platform.binaryName],
    preferUnplugged: true,
  };

  writeFileSync(
    join(platformDir, "package.json"),
    JSON.stringify(packageJson, null, 2) + "\n"
  );

  // Create README
  const readme = `# ${packageName}

Platform-specific binary for [tsgo](https://www.npmjs.com/package/tsgo).

This package contains the native binary for ${platform.os}/${platform.cpu}.

**Do not install this package directly.** Install \`tsgo\` instead:

\`\`\`bash
bun add tsgo
\`\`\`

The correct platform package will be installed automatically.
`;

  writeFileSync(join(platformDir, "README.md"), readme);

  // Copy binary if it exists
  const sourceBinary = join(binariesDir, platform.name, platform.binaryName);
  const destBinary = join(platformDir, platform.binaryName);

  if (existsSync(sourceBinary)) {
    copyFileSync(sourceBinary, destBinary);
    console.log(`  Copied ${platform.binaryName}`);
  } else {
    console.log(`  [WARN] Binary not found: ${sourceBinary}`);
    // Create placeholder
    writeFileSync(destBinary, "# Placeholder - build binary first\n");
  }
}

console.log("");
console.log("Done! Platform packages created in:");
console.log(`  ${outputDir}`);
console.log("");
console.log("To publish:");
for (const platform of platforms) {
  console.log(`  cd ${join(outputDir, platform.name)} && npm publish --access public`);
}
