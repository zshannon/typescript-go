# S3 Private Package Resolution for V3

## Problem

V3 uses `bun install` to resolve dependencies, but private packages like `@flickfyi/core` and `@flickfyi/photon` aren't published to npm. They were previously uploaded to S3 in the v1/v2 versioned node_modules layout. V3 needs a way to install them from S3 alongside public npm packages.

## Design

### package.json contract

A new `resolve-s3` field in package.json lists package names that should be fetched from S3 instead of npm. Versions are read from `dependencies`/`devDependencies` — not duplicated.

```json
{
  "main": "./src/index.ts",
  "dependencies": {
    "@flickfyi/core": "0.0.8",
    "@flickfyi/photon": "0.0.3",
    "zod": "^3.23.0"
  },
  "resolve-s3": ["@flickfyi/core", "@flickfyi/photon"]
}
```

Versions in `resolve-s3` packages must be exact (no ranges) since they map directly to S3 keys.

### S3 tarball layout

Private packages are published as standard npm tarballs at:

```
packages/{package-name}/{version}.tgz
```

Examples:
- `packages/@flickfyi/core/0.0.8.tgz`
- `packages/@flickfyi/photon/0.0.3.tgz`

This lives in the same S3 bucket the server already uses (`S3_BUCKET`), under a `packages/` prefix that doesn't collide with the existing `{version}/*` (v1/v2) or `deps/*` (v3 dep cache) prefixes.

### Install flow change

The `installDeps` function signature changes to accept the parsed `*v3PackageJSON` instead of raw `[]byte`, since it needs access to the `ResolveS3` field. This cascades to `resolveDeps`, which must also receive the parsed struct. Both handlers will now call `parsePackageJSON`. Currently `parsePackageJSON` enforces that `main` is present — this validation must be removed from `parsePackageJSON` and moved to the compile handler, so the typecheck handler can parse without requiring `main`.

Between writing `package.json`/`bun.lock` to the temp directory and running `bun install`, a new step pre-seeds node_modules with S3 packages:

1. Write `package.json` + `bun.lock` to temp dir
2. **Pre-seed S3 packages**: For each package name in `resolve-s3`:
   - Look up the version from `dependencies` or `devDependencies`
   - If not found, return 400 identifying the package
   - If the version does not match strict semver (`^\d+\.\d+\.\d+(-[\w.]+)?$`), return 400: "resolve-s3 package {name} must use an exact version, got {version}"
   - Download `packages/{name}/{version}.tgz` from S3 via `GetObject`
   - Extract the tarball into `{tempDir}/node_modules/{name}/`
3. Run `bun install --frozen-lockfile --ignore-scripts` — bun sees the pre-seeded packages (matching name + version in lockfile), skips them, installs only public npm packages
4. Cache result locally and upload to S3 as before

This only runs on a full cache miss. On local disk or S3 dep cache hit, the cached node_modules already contain both private and public packages from the original install.

### Lockfile requirements

The `bun.lock` must include entries for the private packages. This means the dev-time environment where the lockfile is generated must have access to the private packages (either pre-installed in node_modules or available via a local registry). This is a CI/CD concern for the crayon repo — not a server concern. The server trusts the lockfile as-is.

If a private package has transitive dependencies on other private packages, those must also be listed in `resolve-s3`. Transitive dependencies on public npm packages are handled by bun normally.

### Tarball format

Standard npm tarball: a gzipped tar containing a `package/` directory at the root. This matches what `npm pack` or `yarn pack` produces. The extraction step strips the `package/` prefix and writes contents directly to `node_modules/{name}/`.

Example tarball contents:
```
package/package.json
package/index.js
package/index.d.ts
package/dist/...
```

Extracted to:
```
node_modules/@flickfyi/core/package.json
node_modules/@flickfyi/core/index.js
node_modules/@flickfyi/core/index.d.ts
node_modules/@flickfyi/core/dist/...
```

### v3PackageJSON struct change

Add `ResolveS3` field:

```go
type v3PackageJSON struct {
    Dependencies    map[string]string `json:"dependencies"`
    DevDependencies map[string]string `json:"devDependencies"`
    Esbuild         v3EsbuildConfig   `json:"esbuild"`
    Main            string            `json:"main"`
    ResolveS3       []string          `json:"resolve-s3"`
}
```

### Error handling

- Package listed in `resolve-s3` but not found in `dependencies` or `devDependencies`: return 400 with message identifying the missing package.
- Package listed in `resolve-s3` with a non-exact version (doesn't match strict semver): return 400 with message.
- S3 download failure for a private package (e.g., tarball not found): return 502 with message identifying the package and S3 error.
- Tarball extraction failure: return 502 with details.

### Publishing workflow (out of scope for this server change)

The crayon repo's release workflow for `@flickfyi/core` needs to:
1. Build the package
2. Run `npm pack` (or `yarn pack`) to produce a tarball
3. Upload the tarball to `s3://{bucket}/packages/@flickfyi/core/{version}.tgz`

This is a CI/CD change in the crayon repo, not a server change. The server only consumes the tarballs.

### What does NOT change

- The dep cache key (SHA256 of bun.lock) is unchanged. The lockfile already accounts for private package versions.
- The 3-tier cache lookup (local -> S3 -> install) is unchanged. Pre-seeding only happens during the install step.
- The multipart request format is unchanged.
- The `resolve-s3` field is optional. If absent or empty, the server behaves exactly as before (pure bun install from npm).
