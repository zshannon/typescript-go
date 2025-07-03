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
    public var cValue: Int32 {
        switch self {
        case .default: esbuild_platform_default()
        case .browser: esbuild_platform_browser()
        case .node: esbuild_platform_node()
        case .neutral: esbuild_platform_neutral()
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

// MARK: - Format Enum

/// ESBuild output module format options
public enum ESBuildFormat: Int32, CaseIterable {
    /// Default output format
    case `default`
    /// Immediately Invoked Function Expression
    case iife
    /// CommonJS module format
    case commonjs
    /// ES module format
    case esmodule

    public var cValue: Int32 {
        switch self {
        case .default: esbuild_format_default()
        case .iife: esbuild_format_iife()
        case .commonjs: esbuild_format_commonjs()
        case .esmodule: esbuild_format_esmodule()
        }
    }

    public static func from(cValue: Int32) -> ESBuildFormat? {
        let cases: [(Int32, ESBuildFormat)] = [
            (esbuild_format_default(), .default), (esbuild_format_iife(), .iife),
            (esbuild_format_commonjs(), .commonjs), (esbuild_format_esmodule(), .esmodule),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - Target Enum

/// ESBuild JavaScript target version options
public enum ESBuildTarget: Int32, CaseIterable {
    case `default`, esnext, es5, es2015, es2016, es2017, es2018, es2019, es2020, es2021, es2022, es2023, es2024

    public var cValue: Int32 {
        switch self {
        case .default: esbuild_target_default()
        case .esnext: esbuild_target_esnext()
        case .es5: esbuild_target_es5()
        case .es2015: esbuild_target_es2015()
        case .es2016: esbuild_target_es2016()
        case .es2017: esbuild_target_es2017()
        case .es2018: esbuild_target_es2018()
        case .es2019: esbuild_target_es2019()
        case .es2020: esbuild_target_es2020()
        case .es2021: esbuild_target_es2021()
        case .es2022: esbuild_target_es2022()
        case .es2023: esbuild_target_es2023()
        case .es2024: esbuild_target_es2024()
        }
    }

    public static func from(cValue: Int32) -> ESBuildTarget? {
        let cases: [(Int32, ESBuildTarget)] = [
            (esbuild_target_default(), .default), (esbuild_target_esnext(), .esnext),
            (esbuild_target_es5(), .es5), (esbuild_target_es2015(), .es2015),
            (esbuild_target_es2016(), .es2016), (esbuild_target_es2017(), .es2017),
            (esbuild_target_es2018(), .es2018), (esbuild_target_es2019(), .es2019),
            (esbuild_target_es2020(), .es2020), (esbuild_target_es2021(), .es2021),
            (esbuild_target_es2022(), .es2022), (esbuild_target_es2023(), .es2023),
            (esbuild_target_es2024(), .es2024),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - Loader Enum

/// ESBuild file loader options
public enum ESBuildLoader: Int32, CaseIterable, Sendable {
    case none, base64, binary, copy, css, dataurl, `default`, empty, file, globalcss, js, json, jsx, localcss, text, ts,
         tsx

    public var cValue: Int32 {
        switch self {
        case .none: esbuild_loader_none()
        case .base64: esbuild_loader_base64()
        case .binary: esbuild_loader_binary()
        case .copy: esbuild_loader_copy()
        case .css: esbuild_loader_css()
        case .dataurl: esbuild_loader_dataurl()
        case .default: esbuild_loader_default()
        case .empty: esbuild_loader_empty()
        case .file: esbuild_loader_file()
        case .globalcss: esbuild_loader_globalcss()
        case .js: esbuild_loader_js()
        case .json: esbuild_loader_json()
        case .jsx: esbuild_loader_jsx()
        case .localcss: esbuild_loader_localcss()
        case .text: esbuild_loader_text()
        case .ts: esbuild_loader_ts()
        case .tsx: esbuild_loader_tsx()
        }
    }

    public static func from(cValue: Int32) -> ESBuildLoader? {
        let cases: [(Int32, ESBuildLoader)] = [
            (esbuild_loader_none(), .none), (esbuild_loader_base64(), .base64),
            (esbuild_loader_binary(), .binary), (esbuild_loader_copy(), .copy),
            (esbuild_loader_css(), .css), (esbuild_loader_dataurl(), .dataurl),
            (esbuild_loader_default(), .default), (esbuild_loader_empty(), .empty),
            (esbuild_loader_file(), .file), (esbuild_loader_globalcss(), .globalcss),
            (esbuild_loader_js(), .js), (esbuild_loader_json(), .json),
            (esbuild_loader_jsx(), .jsx), (esbuild_loader_localcss(), .localcss),
            (esbuild_loader_text(), .text), (esbuild_loader_ts(), .ts), (esbuild_loader_tsx(), .tsx),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - SourceMap Enum

/// ESBuild source map generation options
public enum ESBuildSourceMap: Int32, CaseIterable {
    case none, inline, linked, external, inlineAndExternal

    public var cValue: Int32 {
        switch self {
        case .none: esbuild_sourcemap_none()
        case .inline: esbuild_sourcemap_inline()
        case .linked: esbuild_sourcemap_linked()
        case .external: esbuild_sourcemap_external()
        case .inlineAndExternal: esbuild_sourcemap_inlineandexternal()
        }
    }

    public static func from(cValue: Int32) -> ESBuildSourceMap? {
        let cases: [(Int32, ESBuildSourceMap)] = [
            (esbuild_sourcemap_none(), .none), (esbuild_sourcemap_inline(), .inline),
            (esbuild_sourcemap_linked(), .linked), (esbuild_sourcemap_external(), .external),
            (esbuild_sourcemap_inlineandexternal(), .inlineAndExternal),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - JSX Enum

/// ESBuild JSX transformation options
public enum ESBuildJSX: Int32, CaseIterable {
    case transform, preserve, automatic

    public var cValue: Int32 {
        switch self {
        case .transform: esbuild_jsx_transform()
        case .preserve: esbuild_jsx_preserve()
        case .automatic: esbuild_jsx_automatic()
        }
    }

    public static func from(cValue: Int32) -> ESBuildJSX? {
        let cases: [(Int32, ESBuildJSX)] = [
            (esbuild_jsx_transform(), .transform), (esbuild_jsx_preserve(), .preserve),
            (esbuild_jsx_automatic(), .automatic),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - LogLevel Enum

/// ESBuild logging verbosity options
public enum ESBuildLogLevel: Int32, CaseIterable {
    case silent, verbose, debug, info, warning, error

    public var cValue: Int32 {
        switch self {
        case .silent: esbuild_loglevel_silent()
        case .verbose: esbuild_loglevel_verbose()
        case .debug: esbuild_loglevel_debug()
        case .info: esbuild_loglevel_info()
        case .warning: esbuild_loglevel_warning()
        case .error: esbuild_loglevel_error()
        }
    }

    public static func from(cValue: Int32) -> ESBuildLogLevel? {
        let cases: [(Int32, ESBuildLogLevel)] = [
            (esbuild_loglevel_silent(), .silent), (esbuild_loglevel_verbose(), .verbose),
            (esbuild_loglevel_debug(), .debug), (esbuild_loglevel_info(), .info),
            (esbuild_loglevel_warning(), .warning), (esbuild_loglevel_error(), .error),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - LegalComments Enum

/// ESBuild legal comments handling options
public enum ESBuildLegalComments: Int32, CaseIterable {
    case `default`, none, inline, endOfFile, linked, external

    public var cValue: Int32 {
        switch self {
        case .default: esbuild_legalcomments_default()
        case .none: esbuild_legalcomments_none()
        case .inline: esbuild_legalcomments_inline()
        case .endOfFile: esbuild_legalcomments_endoffile()
        case .linked: esbuild_legalcomments_linked()
        case .external: esbuild_legalcomments_external()
        }
    }

    public static func from(cValue: Int32) -> ESBuildLegalComments? {
        let cases: [(Int32, ESBuildLegalComments)] = [
            (esbuild_legalcomments_default(), .default), (esbuild_legalcomments_none(), .none),
            (esbuild_legalcomments_inline(), .inline), (esbuild_legalcomments_endoffile(), .endOfFile),
            (esbuild_legalcomments_linked(), .linked), (esbuild_legalcomments_external(), .external),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - Charset Enum

/// ESBuild character encoding options
public enum ESBuildCharset: Int32, CaseIterable {
    case `default`, ascii, utf8

    public var cValue: Int32 {
        switch self {
        case .default: esbuild_charset_default()
        case .ascii: esbuild_charset_ascii()
        case .utf8: esbuild_charset_utf8()
        }
    }

    public static func from(cValue: Int32) -> ESBuildCharset? {
        let cases: [(Int32, ESBuildCharset)] = [
            (esbuild_charset_default(), .default), (esbuild_charset_ascii(), .ascii),
            (esbuild_charset_utf8(), .utf8),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - TreeShaking Enum

/// ESBuild tree shaking options
public enum ESBuildTreeShaking: Int32, CaseIterable {
    case `default`, `false`, `true`

    public var cValue: Int32 {
        switch self {
        case .default: esbuild_treeshaking_default()
        case .false: esbuild_treeshaking_false()
        case .true: esbuild_treeshaking_true()
        }
    }

    public static func from(cValue: Int32) -> ESBuildTreeShaking? {
        let cases: [(Int32, ESBuildTreeShaking)] = [
            (esbuild_treeshaking_default(), .default), (esbuild_treeshaking_false(), .false),
            (esbuild_treeshaking_true(), .true),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - Color Enum

/// ESBuild stderr color options
public enum ESBuildColor: Int32, CaseIterable {
    case ifTerminal, never, always

    public var cValue: Int32 {
        switch self {
        case .ifTerminal: esbuild_color_ifterminal()
        case .never: esbuild_color_never()
        case .always: esbuild_color_always()
        }
    }

    public static func from(cValue: Int32) -> ESBuildColor? {
        let cases: [(Int32, ESBuildColor)] = [
            (esbuild_color_ifterminal(), .ifTerminal), (esbuild_color_never(), .never),
            (esbuild_color_always(), .always),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - Packages Enum

/// ESBuild packages handling options
public enum ESBuildPackages: Int32, CaseIterable {
    case `default`, bundle, external

    public var cValue: Int32 {
        switch self {
        case .default: esbuild_packages_default()
        case .bundle: esbuild_packages_bundle()
        case .external: esbuild_packages_external()
        }
    }

    public static func from(cValue: Int32) -> ESBuildPackages? {
        let cases: [(Int32, ESBuildPackages)] = [
            (esbuild_packages_default(), .default), (esbuild_packages_bundle(), .bundle),
            (esbuild_packages_external(), .external),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - SourcesContent Enum

/// ESBuild sources content options
public enum ESBuildSourcesContent: Int32, CaseIterable {
    case include, exclude

    public var cValue: Int32 {
        switch self {
        case .include: esbuild_sourcescontent_include()
        case .exclude: esbuild_sourcescontent_exclude()
        }
    }

    public static func from(cValue: Int32) -> ESBuildSourcesContent? {
        let cases: [(Int32, ESBuildSourcesContent)] = [
            (esbuild_sourcescontent_include(), .include), (esbuild_sourcescontent_exclude(), .exclude),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - MangleQuoted Enum

/// ESBuild mangle quoted options
public enum ESBuildMangleQuoted: Int32, CaseIterable {
    case `false`, `true`

    public var cValue: Int32 {
        switch self {
        case .false: esbuild_manglequoted_false()
        case .true: esbuild_manglequoted_true()
        }
    }

    public static func from(cValue: Int32) -> ESBuildMangleQuoted? {
        let cases: [(Int32, ESBuildMangleQuoted)] = [
            (esbuild_manglequoted_false(), .false), (esbuild_manglequoted_true(), .true),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - Drop Enum

/// ESBuild drop options (bit flags)
public enum ESBuildDrop: Int32, CaseIterable {
    case console, debugger

    public var cValue: Int32 {
        switch self {
        case .console: esbuild_drop_console()
        case .debugger: esbuild_drop_debugger()
        }
    }

    public static func from(cValue: Int32) -> ESBuildDrop? {
        let cases: [(Int32, ESBuildDrop)] = [
            (esbuild_drop_console(), .console), (esbuild_drop_debugger(), .debugger),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - Engine Enum

/// ESBuild engine name options
public enum ESBuildEngine: Int32, CaseIterable {
    case chrome, deno, edge, firefox, hermes, ie, ios, node, opera, rhino, safari

    public var cValue: Int32 {
        switch self {
        case .chrome: esbuild_engine_chrome()
        case .deno: esbuild_engine_deno()
        case .edge: esbuild_engine_edge()
        case .firefox: esbuild_engine_firefox()
        case .hermes: esbuild_engine_hermes()
        case .ie: esbuild_engine_ie()
        case .ios: esbuild_engine_ios()
        case .node: esbuild_engine_node()
        case .opera: esbuild_engine_opera()
        case .rhino: esbuild_engine_rhino()
        case .safari: esbuild_engine_safari()
        }
    }

    public static func from(cValue: Int32) -> ESBuildEngine? {
        let cases: [(Int32, ESBuildEngine)] = [
            (esbuild_engine_chrome(), .chrome), (esbuild_engine_deno(), .deno),
            (esbuild_engine_edge(), .edge), (esbuild_engine_firefox(), .firefox),
            (esbuild_engine_hermes(), .hermes), (esbuild_engine_ie(), .ie),
            (esbuild_engine_ios(), .ios), (esbuild_engine_node(), .node),
            (esbuild_engine_opera(), .opera), (esbuild_engine_rhino(), .rhino),
            (esbuild_engine_safari(), .safari),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - SideEffects Enum

/// ESBuild side effects options
public enum ESBuildSideEffects: Int32, CaseIterable {
    case `true`, `false`

    public var cValue: Int32 {
        switch self {
        case .true: esbuild_sideeffects_true()
        case .false: esbuild_sideeffects_false()
        }
    }

    public static func from(cValue: Int32) -> ESBuildSideEffects? {
        let cases: [(Int32, ESBuildSideEffects)] = [
            (esbuild_sideeffects_true(), .true), (esbuild_sideeffects_false(), .false),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - ResolveKind Enum

/// ESBuild resolve kind options
public enum ESBuildResolveKind: Int32, CaseIterable {
    case none, entryPoint, jsImportStatement, jsRequireCall, jsDynamicImport, jsRequireResolve, cssImportRule,
         cssComposesFrom, cssURLToken

    public var cValue: Int32 {
        switch self {
        case .none: esbuild_resolvekind_none()
        case .entryPoint: esbuild_resolvekind_entrypoint()
        case .jsImportStatement: esbuild_resolvekind_jsimportstatement()
        case .jsRequireCall: esbuild_resolvekind_jsrequirecall()
        case .jsDynamicImport: esbuild_resolvekind_jsdynamicimport()
        case .jsRequireResolve: esbuild_resolvekind_jsrequireresolve()
        case .cssImportRule: esbuild_resolvekind_cssimportrule()
        case .cssComposesFrom: esbuild_resolvekind_csscomposesfrom()
        case .cssURLToken: esbuild_resolvekind_cssurltoken()
        }
    }

    public static func from(cValue: Int32) -> ESBuildResolveKind? {
        let cases: [(Int32, ESBuildResolveKind)] = [
            (esbuild_resolvekind_none(), .none), (esbuild_resolvekind_entrypoint(), .entryPoint),
            (esbuild_resolvekind_jsimportstatement(), .jsImportStatement), (
                esbuild_resolvekind_jsrequirecall(),
                .jsRequireCall
            ),
            (esbuild_resolvekind_jsdynamicimport(), .jsDynamicImport), (
                esbuild_resolvekind_jsrequireresolve(),
                .jsRequireResolve
            ),
            (esbuild_resolvekind_cssimportrule(), .cssImportRule), (
                esbuild_resolvekind_csscomposesfrom(),
                .cssComposesFrom
            ),
            (esbuild_resolvekind_cssurltoken(), .cssURLToken),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}

// MARK: - MessageKind Enum

/// ESBuild message kind options
public enum ESBuildMessageKind: Int32, CaseIterable {
    case error, warning

    public var cValue: Int32 {
        switch self {
        case .error: esbuild_messagekind_error()
        case .warning: esbuild_messagekind_warning()
        }
    }

    public static func from(cValue: Int32) -> ESBuildMessageKind? {
        let cases: [(Int32, ESBuildMessageKind)] = [
            (esbuild_messagekind_error(), .error), (esbuild_messagekind_warning(), .warning),
        ]
        return cases.first { $0.0 == cValue }?.1
    }
}
