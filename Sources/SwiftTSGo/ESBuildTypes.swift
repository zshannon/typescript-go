import Foundation
import TSCBridge

/// ESBuild platform targeting options
public enum ESBuildPlatform: Int32, CaseIterable {
    /// Default platform (typically resolves to browser behavior)
    case `default`

    /// Browser platform target
    /// - Wraps code in IIFE to prevent global scope pollution
    /// - Uses browser-specific package resolution
    /// - Defines process.env.NODE_ENV
    case browser

    /// Node.js platform target
    /// - Uses CommonJS format by default
    /// - Marks built-in Node.js modules as external
    /// - Uses Node.js-specific package resolution
    case node

    /// Platform-neutral target
    /// - Uses ECMAScript Module format by default
    /// - No automatic platform-specific behavior
    /// - Maximum flexibility for different runtime environments
    case neutral

    /// Get the actual raw value from the C bridge
    public var actualRawValue: Int32 {
        switch self {
        case .default: return esbuild_platform_default()
        case .browser: return esbuild_platform_browser()
        case .node: return esbuild_platform_node()
        case .neutral: return esbuild_platform_neutral()
        }
    }

    /// Initialize from C bridge value
    public static func from(cValue: Int32) -> ESBuildPlatform? {
        let defaultValue = esbuild_platform_default()
        let browserValue = esbuild_platform_browser()
        let nodeValue = esbuild_platform_node()
        let neutralValue = esbuild_platform_neutral()

        switch cValue {
        case defaultValue: return .default
        case browserValue: return .browser
        case nodeValue: return .node
        case neutralValue: return .neutral
        default: return nil
        }
    }
}

// MARK: - C Bridge Integration

extension ESBuildPlatform {
    /// Get the C bridge integer value for this platform
    /// - Returns: Integer value used by the C bridge
    public var cValue: Int32 {
        return self.actualRawValue
    }
}
