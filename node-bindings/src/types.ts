// TypeScript configuration types matching Swift implementation
export interface DiagnosticInfo {
  code: number;
  category: string;
  message: string;
  file: string;
  line: number;
  column: number;
  length: number;
}

export interface Source {
  name: string;
  content: string;
}

export interface BuildResult {
  success: boolean;
  diagnostics: DiagnosticInfo[];
  compiledFiles: Source[];
  configFile: string;
  writtenFiles: Record<string, string>;
}

export interface BuildOptions {
  printErrors?: boolean;
  configFile?: string;
  workingDirectory?: string;
}

export type FileResolverResult = 
  | { type: 'file'; content: string }
  | { type: 'directory'; files: string[] }
  | null;

export type FileResolver = (path: string) => FileResolverResult | Promise<FileResolverResult>;

// TypeScript compiler configuration enums
export enum ECMAScriptTarget {
  ES3 = "ES3",
  ES5 = "ES5",
  ES2015 = "ES2015",
  ES2016 = "ES2016",
  ES2017 = "ES2017",
  ES2018 = "ES2018",
  ES2019 = "ES2019",
  ES2020 = "ES2020",
  ES2021 = "ES2021",
  ES2022 = "ES2022",
  ESNext = "ESNext"
}

export enum ModuleKind {
  None = "none",
  CommonJS = "commonjs",
  AMD = "amd",
  System = "system",
  UMD = "umd",
  ES6 = "es6",
  ES2015 = "es2015",
  ES2020 = "es2020",
  ES2022 = "es2022",
  ESNext = "esnext",
  Node16 = "node16",
  NodeNext = "nodenext"
}

export enum ModuleResolutionKind {
  Classic = "classic",
  Node = "node",
  Node16 = "node16",
  NodeNext = "nodenext",
  Bundler = "bundler"
}

export enum JSXEmit {
  None = "none",
  Preserve = "preserve",
  React = "react",
  ReactNative = "react-native",
  ReactJSX = "react-jsx",
  ReactJSXDev = "react-jsxdev"
}

export enum NewLineKind {
  CRLF = "crlf",
  LF = "lf"
}

export enum ImportsNotUsedAsValues {
  Remove = "remove",
  Preserve = "preserve",
  Error = "error"
}

export interface CompilerOptions {
  // Type Checking
  allowUnreachableCode?: boolean;
  allowUnusedLabels?: boolean;
  alwaysStrict?: boolean;
  exactOptionalPropertyTypes?: boolean;
  noFallthroughCasesInSwitch?: boolean;
  noImplicitAny?: boolean;
  noImplicitOverride?: boolean;
  noImplicitReturns?: boolean;
  noImplicitThis?: boolean;
  noPropertyAccessFromIndexSignature?: boolean;
  noUncheckedIndexedAccess?: boolean;
  noUnusedLocals?: boolean;
  noUnusedParameters?: boolean;
  strict?: boolean;
  strictBindCallApply?: boolean;
  strictFunctionTypes?: boolean;
  strictNullChecks?: boolean;
  strictPropertyInitialization?: boolean;
  useUnknownInCatchVariables?: boolean;

  // Modules
  allowArbitraryExtensions?: boolean;
  allowImportingTsExtensions?: boolean;
  allowSyntheticDefaultImports?: boolean;
  allowUmdGlobalAccess?: boolean;
  baseUrl?: string;
  customConditions?: string[];
  module?: ModuleKind;
  moduleResolution?: ModuleResolutionKind;
  moduleSuffixes?: string[];
  noResolve?: boolean;
  paths?: Record<string, string[]>;
  resolveJsonModule?: boolean;
  resolvePackageJsonExports?: boolean;
  resolvePackageJsonImports?: boolean;
  rootDir?: string;
  rootDirs?: string[];
  typeRoots?: string[];
  types?: string[];

  // Emit
  declaration?: boolean;
  declarationDir?: string;
  declarationMap?: boolean;
  downlevelIteration?: boolean;
  emitBOM?: boolean;
  emitDeclarationOnly?: boolean;
  emitDecoratorMetadata?: boolean;
  experimentalDecorators?: boolean;
  importHelpers?: boolean;
  importsNotUsedAsValues?: ImportsNotUsedAsValues;
  inlineSourceMap?: boolean;
  inlineSources?: boolean;
  mapRoot?: string;
  newLine?: NewLineKind;
  noEmit?: boolean;
  noEmitHelpers?: boolean;
  noEmitOnError?: boolean;
  outDir?: string;
  outFile?: string;
  preserveConstEnums?: boolean;
  preserveValueImports?: boolean;
  removeComments?: boolean;
  sourceMap?: boolean;
  sourceRoot?: string;
  stripInternal?: boolean;

  // JavaScript Support
  allowJs?: boolean;
  checkJs?: boolean;
  maxNodeModuleJsDepth?: number;

  // Editor Support
  disableSizeLimit?: boolean;
  plugins?: string[];

  // Interop Constraints
  esModuleInterop?: boolean;
  forceConsistentCasingInFileNames?: boolean;
  isolatedModules?: boolean;
  preserveSymlinks?: boolean;
  verbatimModuleSyntax?: boolean;

  // Language and Environment
  jsx?: JSXEmit;
  jsxFactory?: string;
  jsxFragmentFactory?: string;
  jsxImportSource?: string;
  lib?: string[];
  moduleDetection?: string;
  noLib?: boolean;
  reactNamespace?: string;
  target?: ECMAScriptTarget;
  useDefineForClassFields?: boolean;

  // Other options...
  skipLibCheck?: boolean;
}

export interface ProjectReference {
  path: string;
  prepend?: boolean;
  circular?: boolean;
}

export interface TypeAcquisition {
  enable?: boolean;
  include?: string[];
  exclude?: string[];
  disableFilenameBasedTypeAcquisition?: boolean;
}

export interface WatchOptions {
  watchFile?: string;
  watchDirectory?: string;
  fallbackPolling?: string;
  synchronousWatchDirectory?: boolean;
  excludeDirectories?: string[];
  excludeFiles?: string[];
}

export interface TSConfig {
  compilerOptions?: CompilerOptions;
  files?: string[];
  include?: string[];
  exclude?: string[];
  extends?: string;
  references?: ProjectReference[];
  typeAcquisition?: TypeAcquisition;
  watchOptions?: WatchOptions;
}