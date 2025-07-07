//go:build ios

package osvfs

// iOS-specific initialization that avoids calling os.Executable()
// On iOS, we can't get the executable path since we're running as a library
// Default to case-insensitive since iOS file system is typically case-insensitive
var isFileSystemCaseSensitive = false