package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/otel/attribute"
)

// depInstallResult holds the outcome of a dep install operation.
// Multiple concurrent requests for the same hash share one result.
type depInstallResult struct {
	done chan struct{}
	err  error
	path string
}

var (
	depInstallMu       sync.Mutex
	depInstallInFlight = make(map[string]*depInstallResult)
)

// hashBunLock returns the SHA256 hex digest of the bun.lock content.
func hashBunLock(lockContent []byte) string {
	sum := sha256.Sum256(lockContent)
	return hex.EncodeToString(sum[:])
}

// resolveDeps resolves dependencies using a 3-tier lookup:
//  1. Local disk cache
//  2. S3 cache
//  3. bun install
//
// Concurrent requests for the same hash are deduplicated.
func resolveDeps(ctx context.Context, lockContent []byte, pkg *v3PackageJSON, rawPackageJSON []byte) (string, error) {
	ctx, span := startSpan(ctx, "fly_tsgo.deps.resolve",
		attribute.Int("fly_tsgo.deps.resolve_s3.count", len(pkg.ResolveS3)),
	)
	defer span.End()

	hash := hashBunLock(lockContent)
	depDir := filepath.Join(diskCachePath, "deps", hash)

	// Tier 1: local disk check
	nmDir := filepath.Join(depDir, "node_modules")
	if _, err := os.Stat(nmDir); err == nil {
		depCacheLookups.WithLabelValues("disk_hit").Inc()
		span.SetAttributes(attribute.String("fly_tsgo.deps.cache.result", "disk_hit"))
		// Touch mtime for LRU eviction tracking
		now := time.Now()
		_ = os.Chtimes(depDir, now, now)
		return depDir, nil
	}

	// Concurrency dedup: only one goroutine does the install/download per hash
	depInstallMu.Lock()
	if inflight, ok := depInstallInFlight[hash]; ok {
		depInstallMu.Unlock()
		// Wait for the in-flight install to complete
		_, waitSpan := startSpan(ctx, "fly_tsgo.deps.wait_inflight")
		<-inflight.done
		waitSpan.SetAttributes(attribute.Bool("fly_tsgo.deps.wait_inflight.success", inflight.err == nil))
		recordSpanError(waitSpan, "err-deps-inflight", inflight.err)
		waitSpan.End()
		span.SetAttributes(attribute.String("fly_tsgo.deps.cache.result", "inflight"))
		recordSpanError(span, "err-deps-inflight", inflight.err)
		return inflight.path, inflight.err
	}
	result := &depInstallResult{
		done: make(chan struct{}),
	}
	depInstallInFlight[hash] = result
	depInstallMu.Unlock()

	// Perform the resolution (S3 then bun install)
	path, err := func() (string, error) {
		// Tier 2: S3 lookup
		if s3Client != nil {
			p, s3Err := resolveDepsFromS3(ctx, hash, depDir)
			if s3Err == nil {
				depCacheLookups.WithLabelValues("s3_hit").Inc()
				span.SetAttributes(attribute.String("fly_tsgo.deps.cache.result", "s3_hit"))
				return p, nil
			}
		}

		// Tier 3: bun install
		depCacheLookups.WithLabelValues("install").Inc()
		span.SetAttributes(attribute.String("fly_tsgo.deps.cache.result", "install"))
		return installDeps(ctx, hash, depDir, lockContent, pkg, rawPackageJSON)
	}()
	recordSpanError(span, "err-deps-resolve", err)

	// Broadcast result to all waiters
	result.path = path
	result.err = err
	close(result.done)

	// Remove from in-flight map
	depInstallMu.Lock()
	delete(depInstallInFlight, hash)
	depInstallMu.Unlock()

	return path, err
}

// resolveDepsFromS3 downloads and extracts a deps tarball from S3.
func resolveDepsFromS3(ctx context.Context, hash string, depDir string) (string, error) {
	ctx, span := startSpan(ctx, "fly_tsgo.deps.s3_restore")
	defer span.End()

	key := "deps/" + hash + ".tar.gz"
	_, getSpan := startSpan(ctx, "fly_tsgo.s3.get_object",
		attribute.String("fly_tsgo.s3.key_kind", "deps_tarball"),
	)
	out, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		recordSpanError(getSpan, "err-s3-get-deps-tarball", err)
		getSpan.End()
		recordSpanError(span, "err-deps-s3-restore", err)
		return "", fmt.Errorf("s3 get %s: %w", key, err)
	}
	getSpan.End()
	defer out.Body.Close()

	if err := os.MkdirAll(depDir, 0755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", depDir, err)
	}

	if err := extractTarGz(out.Body, depDir); err != nil {
		recordSpanError(span, "err-deps-s3-extract", err)
		return "", fmt.Errorf("extract tar.gz: %w", err)
	}

	return depDir, nil
}

// installDeps runs bun install in a temp dir, moves the result to the cache,
// and uploads the tarball to S3 in the background.
func installDeps(ctx context.Context, hash string, depDir string, lockContent []byte, pkg *v3PackageJSON, rawPackageJSON []byte) (string, error) {
	ctx, span := startSpan(ctx, "fly_tsgo.deps.install",
		attribute.Int("fly_tsgo.deps.resolve_s3.count", len(pkg.ResolveS3)),
	)
	defer span.End()
	start := time.Now()

	tmpDir, err := os.MkdirTemp("", "bun-install-*")
	if err != nil {
		return "", fmt.Errorf("mkdirtemp: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), rawPackageJSON, 0644); err != nil {
		return "", fmt.Errorf("write package.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "bun.lock"), lockContent, 0644); err != nil {
		return "", fmt.Errorf("write bun.lock: %w", err)
	}

	// Pre-seed private S3 packages into node_modules before bun install
	if len(pkg.ResolveS3) > 0 {
		nmDir := filepath.Join(tmpDir, "node_modules")
		if err := os.MkdirAll(nmDir, 0755); err != nil {
			recordSpanError(span, "err-deps-mkdir-node-modules", err)
			return "", fmt.Errorf("mkdir node_modules: %w", err)
		}
		if err := preseedS3Packages(ctx, pkg, nmDir); err != nil {
			recordSpanError(span, "err-deps-preseed-s3-packages", err)
			return "", fmt.Errorf("preseed s3 packages: %w", err)
		}
	}

	_, bunSpan := startSpan(ctx, "fly_tsgo.deps.bun_install")
	cmd := exec.CommandContext(ctx, "bun", "install", "--frozen-lockfile", "--ignore-scripts")
	cmd.Dir = tmpDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		recordSpanError(bunSpan, "err-bun-install", err)
		bunSpan.End()
		recordSpanError(span, "err-bun-install", err)
		return "", fmt.Errorf("bun install failed: %w\nstderr: %s", err, stderr.String())
	}
	bunSpan.End()

	depInstallDuration.Observe(time.Since(start).Seconds())

	// Move node_modules to cache location
	if err := os.MkdirAll(filepath.Dir(depDir), 0755); err != nil {
		return "", fmt.Errorf("mkdir cache parent: %w", err)
	}

	tmpNM := filepath.Join(tmpDir, "node_modules")
	destNM := filepath.Join(depDir, "node_modules")

	// Attempt rename first (fast if same filesystem), fall back to copy
	if err := os.Rename(filepath.Join(tmpDir), depDir); err != nil {
		_, copySpan := startSpan(ctx, "fly_tsgo.deps.cache_copy")
		if err2 := os.MkdirAll(depDir, 0755); err2 != nil {
			recordSpanError(copySpan, "err-deps-mkdir-cache-dir", err2)
			copySpan.End()
			recordSpanError(span, "err-deps-mkdir-cache-dir", err2)
			return "", fmt.Errorf("mkdir depDir: %w", err2)
		}
		if err2 := copyDir(tmpNM, destNM); err2 != nil {
			recordSpanError(copySpan, "err-deps-copy-node-modules", err2)
			copySpan.End()
			recordSpanError(span, "err-deps-copy-node-modules", err2)
			return "", fmt.Errorf("copy node_modules: %w", err2)
		}
		copySpan.End()
	}

	// Upload to S3 in background
	go uploadDepsToS3(context.Background(), hash, depDir)

	return depDir, nil
}

// uploadDepsToS3 creates a tar.gz of node_modules and uploads it to S3.
func uploadDepsToS3(ctx context.Context, hash string, depDir string) {
	if s3Client == nil {
		return
	}
	ctx, span := startSpan(ctx, "fly_tsgo.deps.s3_upload")
	defer span.End()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	nmDir := filepath.Join(depDir, "node_modules")
	err := filepath.Walk(nmDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(depDir, path)
		if err != nil {
			return err
		}
		// Use forward slashes in tar headers
		rel = filepath.ToSlash(rel)

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if !info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		recordSpanError(span, "err-deps-upload-walk", err)
		log.Printf("uploadDepsToS3: walk error for hash %s: %v", hash, err)
		return
	}

	if err := tw.Close(); err != nil {
		recordSpanError(span, "err-deps-upload-tar-close", err)
		log.Printf("uploadDepsToS3: tw.Close error: %v", err)
		return
	}
	if err := gw.Close(); err != nil {
		recordSpanError(span, "err-deps-upload-gzip-close", err)
		log.Printf("uploadDepsToS3: gw.Close error: %v", err)
		return
	}

	key := "deps/" + hash + ".tar.gz"
	_, putSpan := startSpan(ctx, "fly_tsgo.s3.put_object",
		attribute.Int("fly_tsgo.s3.body.bytes", buf.Len()),
		attribute.String("fly_tsgo.s3.key_kind", "deps_tarball"),
	)
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s3Bucket),
		Body:   bytes.NewReader(buf.Bytes()),
		Key:    aws.String(key),
	})
	if err != nil {
		recordSpanError(putSpan, "err-s3-put-deps-tarball", err)
		putSpan.End()
		recordSpanError(span, "err-s3-put-deps-tarball", err)
		log.Printf("uploadDepsToS3: PutObject error for %s: %v", key, err)
		return
	}
	putSpan.End()
}

// extractTarGz extracts a tar.gz stream into destDir.
func extractTarGz(r io.Reader, destDir string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar next: %w", err)
		}

		// Sanitize path to prevent directory traversal
		cleanName := filepath.Clean(header.Name)
		if filepath.IsAbs(cleanName) {
			continue
		}

		target := filepath.Join(destDir, cleanName)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)|0755); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", target, err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode)|0644)
			if err != nil {
				return fmt.Errorf("create %s: %w", target, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("write %s: %w", target, err)
			}
			f.Close()
		}
	}
	return nil
}

// copyDir recursively copies src directory to dst.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target, info.Mode())
	})
}

// copyFile copies a single file from src to dst with the given mode.
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// exactSemverRe matches exact semver versions like 1.2.3 or 1.2.3-beta.1
var exactSemverRe = regexp.MustCompile(`^\d+\.\d+\.\d+(-[\w.]+)?$`)

// lookupS3PackageVersion finds the version for a resolve-s3 package in deps or devDeps.
// Only exact semver versions are allowed (no ranges like ^1.0.0).
func lookupS3PackageVersion(pkg *v3PackageJSON, name string) (string, error) {
	version := pkg.Dependencies[name]
	if version == "" {
		version = pkg.DevDependencies[name]
	}
	if version == "" {
		return "", fmt.Errorf("resolve-s3 package %q not found in dependencies or devDependencies", name)
	}
	if !exactSemverRe.MatchString(version) {
		return "", fmt.Errorf("resolve-s3 package %q must use an exact version, got %q", name, version)
	}
	return version, nil
}

// extractNpmTarball extracts a gzipped npm tarball into destDir,
// stripping the leading "package/" prefix from all entries.
func extractNpmTarball(r io.Reader, destDir string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar next: %w", err)
		}

		// Strip "package/" prefix (npm tarballs wrap everything in package/)
		name := header.Name
		if strings.HasPrefix(name, "package/") {
			name = strings.TrimPrefix(name, "package/")
		}
		if name == "" || name == "." {
			continue
		}

		// Sanitize path to prevent directory traversal
		cleanName := filepath.Clean(name)
		if filepath.IsAbs(cleanName) {
			continue
		}

		target := filepath.Join(destDir, cleanName)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)|0755); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", target, err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode)|0644)
			if err != nil {
				return fmt.Errorf("create %s: %w", target, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("write %s: %w", target, err)
			}
			f.Close()
		}
	}
	return nil
}

// preseedS3Packages downloads private packages from S3 and extracts them
// into node_modules before bun install runs.
func preseedS3Packages(ctx context.Context, pkg *v3PackageJSON, nodeModulesDir string) error {
	if s3Client == nil {
		return fmt.Errorf("s3 client not configured")
	}

	ctx, span := startSpan(ctx, "fly_tsgo.deps.preseed_s3_packages",
		attribute.Int("fly_tsgo.deps.resolve_s3.count", len(pkg.ResolveS3)),
	)
	defer span.End()

	for _, name := range pkg.ResolveS3 {
		version, err := lookupS3PackageVersion(pkg, name)
		if err != nil {
			recordSpanError(span, "err-preseed-s3-version", err)
			return err
		}

		key := "packages/" + name + "/" + version + ".tgz"
		_, packageSpan := startSpan(ctx, "fly_tsgo.deps.preseed_s3_package",
			attribute.String("fly_tsgo.package.name", name),
		)
		out, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(s3Bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			recordSpanError(packageSpan, "err-s3-get-private-package", err)
			packageSpan.End()
			recordSpanError(span, "err-s3-get-private-package", err)
			return fmt.Errorf("s3 get %s: %w", key, err)
		}
		defer out.Body.Close()

		pkgDir := filepath.Join(nodeModulesDir, name)
		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			recordSpanError(packageSpan, "err-preseed-package-mkdir", err)
			packageSpan.End()
			recordSpanError(span, "err-preseed-package-mkdir", err)
			return fmt.Errorf("mkdir %s: %w", pkgDir, err)
		}

		if err := extractNpmTarball(out.Body, pkgDir); err != nil {
			recordSpanError(packageSpan, "err-preseed-package-extract", err)
			packageSpan.End()
			recordSpanError(span, "err-preseed-package-extract", err)
			return fmt.Errorf("extract %s: %w", name, err)
		}
		packageSpan.End()
	}

	return nil
}
