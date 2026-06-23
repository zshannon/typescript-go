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
	"golang.org/x/sync/errgroup"
)

const depsS3Prefix = "deps/"

// depInstallResult holds the outcome of a dep install operation.
// Multiple concurrent requests for the same hash share one result.
type depInstallResult struct {
	done    chan struct{}
	err     error
	path    string
	ready   chan struct{}
	tempDir string
}

var (
	depInstallMu       sync.Mutex
	depInstallInFlight = make(map[string]*depInstallResult)
)

func newDepInstallResult() *depInstallResult {
	return &depInstallResult{
		done:  make(chan struct{}),
		ready: make(chan struct{}),
	}
}

// hashBunLock returns the SHA256 hex digest of the bun.lock content.
func hashBunLock(lockContent []byte) string {
	sum := sha256.Sum256(lockContent)
	return hex.EncodeToString(sum[:])
}

func depsCacheDir(hash string) string {
	return filepath.Join(diskCachePath, "deps", hash)
}

func depsCacheS3Key(hash string) string {
	return depsS3Prefix + hash + ".tar.gz"
}

func depsInstallTempRoot() string {
	return filepath.Join(diskCachePath, ".deps-tmp")
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
	depDir := depsCacheDir(hash)

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
		// Wait until the dependency directory is usable. The full lifecycle may
		// continue while the cache tarball uploads in the background.
		if err := waitForDepInstallReady(ctx, inflight); err != nil {
			recordSpanError(span, "err-deps-inflight-wait", err)
			return "", err
		}
		span.SetAttributes(
			attribute.String("fly_tsgo.deps.cache.result", "inflight"),
			attribute.Bool("fly_tsgo.deps.wait_inflight.success", inflight.err == nil),
		)
		recordSpanError(span, "err-deps-inflight", inflight.err)
		return inflight.path, inflight.err
	}
	result := newDepInstallResult()
	depInstallInFlight[hash] = result
	depInstallMu.Unlock()

	// Perform the resolution (S3 then bun install)
	uploadAfterReady := false
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
		uploadAfterReady = true
		return installDeps(ctx, hash, depDir, lockContent, pkg, rawPackageJSON, result)
	}()
	recordSpanError(span, "err-deps-resolve", err)

	// Broadcast the usable dependency directory to compile/typecheck waiters.
	result.path = path
	result.err = err
	close(result.ready)
	finishDepInstallAsync(ctx, hash, result, uploadAfterReady)

	return path, err
}

func finishDepInstallAsync(ctx context.Context, hash string, result *depInstallResult, uploadAfterReady bool) {
	if result.err != nil || result.path == "" || !uploadAfterReady {
		closeDepInstallResult(hash, result)
		return
	}

	go func() {
		uploadDepsToS3(ctx, hash, result.path)
		closeDepInstallResult(hash, result)
	}()
}

func closeDepInstallResult(hash string, result *depInstallResult) {
	depInstallMu.Lock()
	if depInstallInFlight[hash] == result {
		delete(depInstallInFlight, hash)
	}
	depInstallMu.Unlock()

	close(result.done)
}

func waitForAllDepInstalls(ctx context.Context) error {
	depInstallMu.Lock()
	inflights := make([]*depInstallResult, 0, len(depInstallInFlight))
	for _, inflight := range depInstallInFlight {
		inflights = append(inflights, inflight)
	}
	depInstallMu.Unlock()

	for _, inflight := range inflights {
		if err := waitForDepInstallResult(ctx, inflight); err != nil {
			return err
		}
	}

	return nil
}

func waitForDepInstall(ctx context.Context, hash string) error {
	depInstallMu.Lock()
	inflight := depInstallInFlight[hash]
	depInstallMu.Unlock()

	if inflight == nil {
		return nil
	}

	return waitForDepInstallResult(ctx, inflight)
}

func waitForDepInstallReady(ctx context.Context, result *depInstallResult) error {
	select {
	case <-result.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func waitForDepInstallResult(ctx context.Context, result *depInstallResult) error {
	select {
	case <-result.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func activeDepInstallTempDirs() map[string]struct{} {
	depInstallMu.Lock()
	defer depInstallMu.Unlock()

	active := make(map[string]struct{}, len(depInstallInFlight))
	for _, inflight := range depInstallInFlight {
		if inflight.tempDir != "" {
			active[filepath.Clean(inflight.tempDir)] = struct{}{}
		}
	}
	return active
}

func setDepInstallTempDir(result *depInstallResult, tmpDir string) {
	if result == nil {
		return
	}
	depInstallMu.Lock()
	result.tempDir = tmpDir
	depInstallMu.Unlock()
}

// resolveDepsFromS3 downloads and extracts a deps tarball from S3.
func resolveDepsFromS3(ctx context.Context, hash string, depDir string) (string, error) {
	key := depsCacheS3Key(hash)
	out, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", fmt.Errorf("s3 get %s: %w", key, err)
	}
	defer out.Body.Close()

	if err := os.MkdirAll(depDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", depDir, err)
	}

	if err := extractTarGz(out.Body, depDir); err != nil {
		return "", fmt.Errorf("extract tar.gz: %w", err)
	}

	return depDir, nil
}

// installDeps runs bun install in a temp dir and moves the result to the cache.
func installDeps(ctx context.Context, hash string, depDir string, lockContent []byte, pkg *v3PackageJSON, rawPackageJSON []byte, result *depInstallResult) (string, error) {
	ctx, span := startSpan(ctx, "fly_tsgo.deps.install",
		attribute.Int("fly_tsgo.deps.resolve_s3.count", len(pkg.ResolveS3)),
	)
	defer span.End()
	start := time.Now()

	tmpRoot := depsInstallTempRoot()
	if err := os.MkdirAll(tmpRoot, 0755); err != nil {
		return "", fmt.Errorf("mkdir temp root: %w", err)
	}

	tmpDir, err := os.MkdirTemp(tmpRoot, "bun-install-*")
	if err != nil {
		return "", fmt.Errorf("mkdirtemp: %w", err)
	}
	setDepInstallTempDir(result, tmpDir)
	defer setDepInstallTempDir(result, "")
	defer os.RemoveAll(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), rawPackageJSON, 0o644); err != nil {
		return "", fmt.Errorf("write package.json: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "bun.lock"), lockContent, 0o644); err != nil {
		return "", fmt.Errorf("write bun.lock: %w", err)
	}

	// Pre-seed private S3 packages into node_modules before bun install
	if len(pkg.ResolveS3) > 0 {
		nmDir := filepath.Join(tmpDir, "node_modules")
		if err := os.MkdirAll(nmDir, 0o755); err != nil {
			recordSpanError(span, "err-deps-mkdir-node-modules", err)
			return "", fmt.Errorf("mkdir node_modules: %w", err)
		}
		if err := preseedS3Packages(ctx, pkg, nmDir); err != nil {
			recordSpanError(span, "err-deps-preseed-s3-packages", err)
			return "", fmt.Errorf("preseed s3 packages: %w", err)
		}
	}

	bunStart := time.Now()
	cmd := exec.CommandContext(ctx, "bun", "install", "--frozen-lockfile", "--ignore-scripts")
	cmd.Dir = tmpDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		span.SetAttributes(attribute.Float64("fly_tsgo.deps.bun_install.duration_ms", spanDurationMS(time.Since(bunStart))))
		recordSpanError(span, "err-bun-install", err)
		return "", fmt.Errorf("bun install failed: %w\nstderr: %s", err, stderr.String())
	}
	span.SetAttributes(attribute.Float64("fly_tsgo.deps.bun_install.duration_ms", spanDurationMS(time.Since(bunStart))))

	depInstallDuration.Observe(time.Since(start).Seconds())

	// Move node_modules to cache location
	if err := os.MkdirAll(filepath.Dir(depDir), 0o755); err != nil {
		return "", fmt.Errorf("mkdir cache parent: %w", err)
	}

	tmpNM := filepath.Join(tmpDir, "node_modules")
	destNM := filepath.Join(depDir, "node_modules")

	if err := os.RemoveAll(depDir); err != nil {
		recordSpanError(span, "err-deps-remove-stale-cache-dir", err)
		return "", fmt.Errorf("remove stale depDir: %w", err)
	}

	// Attempt rename first (fast if same filesystem), fall back to copy
	if err := os.Rename(filepath.Join(tmpDir), depDir); err != nil {
		if err2 := os.MkdirAll(depDir, 0o755); err2 != nil {
			recordSpanError(span, "err-deps-mkdir-cache-dir", err2)
			return "", fmt.Errorf("mkdir depDir: %w", err2)
		}
		if err2 := copyDir(tmpNM, destNM); err2 != nil {
			recordSpanError(span, "err-deps-copy-node-modules", err2)
			return "", fmt.Errorf("copy node_modules: %w", err2)
		}
		span.SetAttributes(attribute.Bool("fly_tsgo.deps.cache.copied", true))
	} else {
		span.SetAttributes(attribute.Bool("fly_tsgo.deps.cache.copied", false))
	}

	return depDir, nil
}

// uploadDepsToS3 creates a tar.gz of node_modules and uploads it to S3.
func uploadDepsToS3(ctx context.Context, hash string, depDir string) {
	if s3Client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
	defer cancel()

	ctx, span := startSpan(ctx, "fly_tsgo.deps.s3_upload")
	defer span.End()

	pr, pw := io.Pipe()
	writeResult := make(chan tarGzWriteResult, 1)
	go func() {
		result := writeDepsTarGz(pw, depDir)
		if result.err != nil {
			_ = pw.CloseWithError(result.err)
		} else {
			result.err = pw.Close()
		}
		writeResult <- result
	}()

	key := "deps/" + hash + ".tar.gz"
	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s3Bucket),
		Body:   pr,
		Key:    aws.String(key),
	})
	if err != nil {
		_ = pr.CloseWithError(err)
	}
	result := <-writeResult
	if result.bytes > 0 {
		span.SetAttributes(attribute.Int64("fly_tsgo.s3.body.bytes", result.bytes))
	}
	if err != nil {
		recordSpanError(span, "err-s3-put-deps-tarball", err)
		log.Printf("uploadDepsToS3: PutObject error for %s: %v", key, err)
		return
	}
	if result.err != nil {
		recordSpanError(span, "err-deps-upload-tarball-stream", result.err)
		log.Printf("uploadDepsToS3: tar.gz stream error for %s: %v", key, result.err)
	}
}

type tarGzWriteResult struct {
	bytes int64
	err   error
}

type countingWriter struct {
	w     io.Writer
	bytes int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.bytes += int64(n)
	return n, err
}

func writeDepsTarGz(w io.Writer, depDir string) tarGzWriteResult {
	cw := &countingWriter{w: w}
	gw := gzip.NewWriter(cw)
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
			if _, err := io.Copy(tw, f); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		_ = tw.Close()
		_ = gw.Close()
		return tarGzWriteResult{bytes: cw.bytes, err: err}
	}

	if err := tw.Close(); err != nil {
		_ = gw.Close()
		return tarGzWriteResult{bytes: cw.bytes, err: err}
	}
	if err := gw.Close(); err != nil {
		return tarGzWriteResult{bytes: cw.bytes, err: err}
	}
	return tarGzWriteResult{bytes: cw.bytes}
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
			if err := os.MkdirAll(target, os.FileMode(header.Mode)|0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", target, err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode)|0o644)
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

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
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
			if err := os.MkdirAll(target, os.FileMode(header.Mode)|0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", target, err)
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode)|0o644)
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

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(4)
	for _, name := range pkg.ResolveS3 {
		name := name
		g.Go(func() error {
			return preseedS3Package(ctx, pkg, nodeModulesDir, name)
		})
	}

	return g.Wait()
}

func preseedS3Package(ctx context.Context, pkg *v3PackageJSON, nodeModulesDir string, name string) error {
	version, err := lookupS3PackageVersion(pkg, name)
	if err != nil {
		return err
	}

	key := "packages/" + name + "/" + version + ".tgz"
	out, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3 get %s: %w", key, err)
	}
	defer out.Body.Close()

	pkgDir := filepath.Join(nodeModulesDir, name)
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", pkgDir, err)
	}

	if err := extractNpmTarball(out.Body, pkgDir); err != nil {
		return fmt.Errorf("extract %s: %w", name, err)
	}

	return nil
}
