package tsoptions_test

import (
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/diagnosticwriter"
	"github.com/microsoft/typescript-go/internal/jsonutil"
	"github.com/microsoft/typescript-go/internal/parser"
	"github.com/microsoft/typescript-go/internal/repo"
	"github.com/microsoft/typescript-go/internal/testutil/baseline"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/tsoptions/tsoptionstest"
	"github.com/microsoft/typescript-go/internal/tspath"
	"github.com/microsoft/typescript-go/internal/vfs"
	"github.com/microsoft/typescript-go/internal/vfs/osvfs"
	"gotest.tools/v3/assert"
)

type testConfig struct {
	jsonText       string
	configFileName string
	basePath       string
	allFileList    map[string]string
}

var parseConfigFileTextToJsonTests = []struct {
	title string
	input []string
}{
	{
		title: "returns empty config for file with only whitespaces",
		input: []string{
			"",
			" ",
		},
	},
	{
		title: "returns empty config for file with comments only",
		input: []string{
			"// Comment",
			"/* Comment*/",
		},
	},
	{
		title: "returns empty config when config is empty object",
		input: []string{
			`{}`,
		},
	},
	{
		title: "returns config object without comments",
		input: []string{
			`{ // Excluded files
            "exclude": [
                // Exclude d.ts
                "file.d.ts"
            ]
        }`,
			`{
            /* Excluded
                    Files
            */
            "exclude": [
                /* multiline comments can be in the middle of a line */"file.d.ts"
            ]
        }`,
		},
	},
	{
		title: "keeps string content untouched",
		input: []string{
			`{
            "exclude": [
                "xx//file.d.ts"
            ]
        }`,
			`{
            "exclude": [
                "xx/*file.d.ts*/"
            ]
        }`,
		},
	},
	{
		title: "handles escaped characters in strings correctly",
		input: []string{
			`{
            "exclude": [
                "xx\"//files"
            ]
        }`,
			`{
            "exclude": [
                "xx\\" // end of line comment
            ]
        }`,
		},
	},
	{
		title: "returns object when users correctly specify library",
		input: []string{
			`{
            "compilerOptions": {
                "lib": ["es5"]
            }
        }`,
			`{
            "compilerOptions": {
                "lib": ["es5", "es6"]
            }
        }`,
		},
	},
}

func TestParseConfigFileTextToJson(t *testing.T) {
	t.Parallel()
	repo.SkipIfNoTypeScriptSubmodule(t)
	for _, rec := range parseConfigFileTextToJsonTests {
		t.Run(rec.title, func(t *testing.T) {
			t.Parallel()
			var baselineContent strings.Builder
			for i, jsonText := range rec.input {
				baselineContent.WriteString("Input::\n")
				baselineContent.WriteString(jsonText + "\n")
				parsed, errors := tsoptions.ParseConfigFileTextToJson("/apath/tsconfig.json", "/apath", jsonText)
				baselineContent.WriteString("Config::\n")
				assert.NilError(t, writeJsonReadableText(&baselineContent, parsed), "Failed to write JSON text")
				baselineContent.WriteString("\n")
				baselineContent.WriteString("Errors::\n")
				diagnosticwriter.FormatDiagnosticsWithColorAndContext(&baselineContent, errors, &diagnosticwriter.FormattingOptions{
					NewLine: "\n",
					ComparePathsOptions: tspath.ComparePathsOptions{
						CurrentDirectory:          "/",
						UseCaseSensitiveFileNames: true,
					},
				})
				baselineContent.WriteString("\n")
				if i != len(rec.input)-1 {
					baselineContent.WriteString("\n")
				}
			}
			baseline.RunAgainstSubmodule(t, rec.title+" jsonParse.js", baselineContent.String(), baseline.Options{Subfolder: "config/tsconfigParsing"})
		})
	}
}

type parseJsonConfigTestCase struct {
	title               string
	noSubmoduleBaseline bool
	input               []testConfig
}

var parseJsonConfigFileTests = []parseJsonConfigTestCase{
	{
		title: "ignore dotted files and folders",
		input: []testConfig{{
			jsonText:       `{}`,
			configFileName: "tsconfig.json",
			basePath:       "/apath",
			allFileList:    map[string]string{"/apath/test.ts": "", "/apath/.git/a.ts": "", "/apath/.b.ts": "", "/apath/..c.ts": ""},
		}},
	},
	{
		title: "allow dotted files and folders when explicitly requested",
		input: []testConfig{{
			jsonText: `{
                    "files": ["/apath/.git/a.ts", "/apath/.b.ts", "/apath/..c.ts"]
                }`,
			configFileName: "tsconfig.json",
			basePath:       "/apath",
			allFileList:    map[string]string{"/apath/test.ts": "", "/apath/.git/a.ts": "", "/apath/.b.ts": "", "/apath/..c.ts": ""},
		}},
	},
	{
		title: "implicitly exclude common package folders",
		input: []testConfig{{
			jsonText:       `{}`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList:    map[string]string{"/node_modules/a.ts": "", "/bower_components/b.ts": "", "/jspm_packages/c.ts": "", "/d.ts": "", "/folder/e.ts": ""},
		}},
	},
	{
		title: "generates errors for empty files list",
		input: []testConfig{{
			jsonText: `{
                "files": []
            }`,
			configFileName: "/apath/tsconfig.json",
			basePath:       "/apath",
			allFileList:    map[string]string{"/apath/a.ts": ""},
		}},
	},
	{
		title: "generates errors for empty files list when no references are provided",
		input: []testConfig{{
			jsonText: `{
                "files": [],
                "references": []
            }`,
			configFileName: "/apath/tsconfig.json",
			basePath:       "/apath",
			allFileList:    map[string]string{"/apath/a.ts": ""},
		}},
	},
	{
		title: "generates errors for directory with no .ts files",
		input: []testConfig{{
			jsonText: `{
            }`,
			configFileName: "/apath/tsconfig.json",
			basePath:       "/apath",
			allFileList:    map[string]string{"/apath/a.js": ""},
		}},
	},
	{
		title: "generates errors for empty include",
		input: []testConfig{{
			jsonText: `{
                "include": []
            }`,
			configFileName: "/apath/tsconfig.json",
			basePath:       "tests/cases/unittests",
			allFileList:    map[string]string{"/apath/a.ts": ""},
		}},
	},
	{
		title:               "parses tsconfig with compilerOptions, files, include, and exclude",
		noSubmoduleBaseline: true,
		input: []testConfig{{
			jsonText: `{
  "compilerOptions": {
    "outDir": "./dist",
    "strict": true,
    "noImplicitAny": true,
    "target": "ES2017",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "moduleDetection": "auto",
    "jsx": "react",
	"maxNodeModuleJsDepth": 1,
	"paths": {
      "jquery": ["./vendor/jquery/dist/jquery"]
    }
  },
  "files": ["/apath/src/index.ts", "/apath/src/app.ts"],
  "include": ["/apath/src/**/*"],
  "exclude": ["/apath/node_modules", "/apath/dist"]
}`,
			configFileName: "/apath/tsconfig.json",
			basePath:       "/apath",
			allFileList:    map[string]string{"/apath/src/index.ts": "", "/apath/src/app.ts": "", "/apath/node_modules/module.ts": "", "/apath/dist/output.js": ""},
		}},
	},
	{
		title: "generates errors when commandline option is in tsconfig",
		input: []testConfig{{
			jsonText: `{
  "compilerOptions": {
    "help": true
  }
}`,
			configFileName: "/apath/tsconfig.json",
			basePath:       "/apath",
			allFileList:    map[string]string{"/apath/a.ts": ""},
		}},
	},
	{
		title: "does not generate errors for empty files list when one or more references are provided",
		input: []testConfig{{
			jsonText: `{
                "files": [],
                "references": [{ "path": "/apath" }]
            }`,
			configFileName: "/apath/tsconfig.json",
			basePath:       "/apath",
			allFileList:    map[string]string{"/apath/a.ts": ""},
		}},
	},
	{
		title: "exclude outDir unless overridden",
		input: []testConfig{{
			jsonText: `{
                "compilerOptions": {
                    "outDir": "bin"
                }
            }`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList:    map[string]string{"/bin/a.ts": "", "/b.ts": ""},
		}, {
			jsonText: `{
                "compilerOptions": {
                    "outDir": "bin"
                },
                "exclude": [ "obj" ]
            }`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList:    map[string]string{"/bin/a.ts": "", "/b.ts": ""},
		}},
	},
	{
		title: "exclude declarationDir unless overridden",
		input: []testConfig{{
			jsonText: `{
                "compilerOptions": {
                    "declarationDir": "declarations"
                }
            }`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList:    map[string]string{"/declarations/a.d.ts": "", "/a.ts": ""},
		}, {
			jsonText: `{
                "compilerOptions": {
                    "declarationDir": "declarations"
                },
                "exclude": [ "types" ]
            }`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList:    map[string]string{"/declarations/a.d.ts": "", "/a.ts": ""},
		}},
	},
	{
		title: "generates errors for empty directory",
		input: []testConfig{{
			jsonText: `{
                "compilerOptions": {
                    "allowJs": true
                }
            }`,
			configFileName: "/apath/tsconfig.json",
			basePath:       "/apath",
			allFileList:    map[string]string{},
		}},
	},
	{
		title: "generates errors for includes with outDir",
		input: []testConfig{{
			jsonText: `{
                "compilerOptions": {
                    "outDir": "./"
                },
                "include": ["**/*"]
            }`,
			configFileName: "/apath/tsconfig.json",
			basePath:       "/apath",
			allFileList:    map[string]string{"/apath/a.ts": ""},
		}},
	},
	{
		title: "generates errors when include is not string",
		input: []testConfig{{
			jsonText: `{
  "include": [
    [
      "./**/*.ts"
    ]
  ]
}`,
			configFileName: "/apath/tsconfig.json",
			basePath:       "/apath",
			allFileList:    map[string]string{"/apath/a.ts": ""},
		}},
	},
	{
		title: "generates errors when files is not string",
		input: []testConfig{{
			jsonText: `{
  "files": [
    {
      "compilerOptions": {
        "experimentalDecorators": true,
        "allowJs": true
      }
    }
  ]
}`,
			configFileName: "/apath/tsconfig.json",
			basePath:       "/apath",
			allFileList:    map[string]string{"/apath/a.ts": ""},
		}},
	},
	{
		title: "with outDir from base tsconfig",
		input: []testConfig{
			{
				jsonText: `{
  "extends": "./tsconfigWithoutConfigDir.json"
}`,
				configFileName: "tsconfig.json",
				basePath:       "/",
				allFileList: map[string]string{
					"/tsconfigWithoutConfigDir.json": tsconfigWithoutConfigDir,
					"/bin/a.ts":                      "",
					"/b.ts":                          "",
				},
			},
			{
				jsonText: `{
  "extends": "./tsconfigWithConfigDir.json"
}`,
				configFileName: "tsconfig.json",
				basePath:       "/",
				allFileList: map[string]string{
					"/tsconfigWithConfigDir.json": tsconfigWithConfigDir,
					"/bin/a.ts":                   "",
					"/b.ts":                       "",
				},
			},
		},
	},
	{
		title: "returns error when tsconfig have excludes",
		input: []testConfig{{
			jsonText: `{
                    "compilerOptions": {
                        "lib": ["es5"]
                    },
                    "excludes": [
                        "foge.ts"
                    ]
                }`,
			configFileName: "tsconfig.json",
			basePath:       "/apath",
			allFileList:    map[string]string{"/apath/test.ts": "", "/apath/foge.ts": ""},
		}},
	},
	{
		title:               "parses tsconfig with extends, files, include and other options",
		noSubmoduleBaseline: true,
		input: []testConfig{{
			jsonText: `{
				"extends": "./tsconfigWithExtends.json",
				"compilerOptions": {
				    "outDir": "./dist",
    				"strict": true,
    				"noImplicitAny": true,
					"baseUrl": "",
				},
			}`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList:    map[string]string{"/tsconfigWithExtends.json": tsconfigWithExtends, "/src/index.ts": "", "/src/app.ts": "", "/node_modules/module.ts": "", "/dist/output.js": ""},
		}},
	},
	{
		title:               "parses tsconfig with extends and configDir",
		noSubmoduleBaseline: true,
		input: []testConfig{{
			jsonText: `{
				"extends": "./tsconfig.base.json"
			}`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList:    map[string]string{"/tsconfig.base.json": tsconfigWithExtendsAndConfigDir, "/src/index.ts": "", "/src/app.ts": "", "/node_modules/module.ts": "", "/dist/output.js": ""},
		}},
	},
	{
		title: "reports error for an unknown option",
		input: []testConfig{{
			jsonText: `{
			    "compilerOptions": {
				"unknown": true
			    }
			}`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList:    map[string]string{"/app.ts": ""},
		}},
	},
	{
		title: "reports errors for wrong type option and invalid enum value",
		input: []testConfig{{
			jsonText: `{
			    "compilerOptions": {
				"target": "invalid value",
				"removeComments": "should be a boolean",
				"moduleResolution": "invalid value"
			    }
			}`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList:    map[string]string{"/app.ts": ""},
		}},
	},
	{
		title:               "handles empty types array",
		noSubmoduleBaseline: true,
		input: []testConfig{{
			jsonText: `{
			    "compilerOptions": {
					"types": []
				}
			}`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList:    map[string]string{"/app.ts": ""},
		}},
	},
	{
		title:               "issue 1267 scenario - extended files not picked up",
		noSubmoduleBaseline: true,
		input: []testConfig{{
			jsonText: `{
  "extends": "./tsconfig-base/backend.json",
  "compilerOptions": {
    "baseUrl": "./",
    "outDir": "dist",
    "rootDir": "src",
    "resolveJsonModule": true
  },
  "exclude": ["node_modules", "dist"],
  "include": ["src/**/*"]
}`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList: map[string]string{
				"/tsconfig-base/backend.json": `{
  "$schema": "https://json.schemastore.org/tsconfig",
  "display": "Backend",
  "compilerOptions": {
    "allowJs": true,
    "module": "nodenext",
    "removeComments": true,
    "emitDecoratorMetadata": true,
    "experimentalDecorators": true,
    "allowSyntheticDefaultImports": true,
    "target": "esnext",
    "lib": ["ESNext"],
    "incremental": false,
    "esModuleInterop": true,
    "noImplicitAny": true,
    "moduleResolution": "nodenext",
    "types": ["node", "vitest/globals"],
    "sourceMap": true,
    "strictPropertyInitialization": false
  },
  "files": [
    "types/ical2json.d.ts",
    "types/express.d.ts",
    "types/multer.d.ts",
    "types/reset.d.ts",
    "types/stripe-custom-typings.d.ts",
    "types/nestjs-modules.d.ts",
    "types/luxon.d.ts",
    "types/nestjs-pino.d.ts"
  ],
  "ts-node": {
    "files": true
  }
}`,
				"/tsconfig-base/types/ical2json.d.ts":             "export {}",
				"/tsconfig-base/types/express.d.ts":               "export {}",
				"/tsconfig-base/types/multer.d.ts":                "export {}",
				"/tsconfig-base/types/reset.d.ts":                 "export {}",
				"/tsconfig-base/types/stripe-custom-typings.d.ts": "export {}",
				"/tsconfig-base/types/nestjs-modules.d.ts":        "export {}",
				"/tsconfig-base/types/luxon.d.ts": `declare module 'luxon' {
  interface TSSettings {
    throwOnInvalid: true
  }
}
export {}`,
				"/tsconfig-base/types/nestjs-pino.d.ts": "export {}",
				"/src/main.ts":                          "export {}",
				"/src/utils.ts":                         "export {}",
			},
		}},
	},
	{
		title:               "null overrides in extended tsconfig - array fields",
		noSubmoduleBaseline: true,
		input: []testConfig{{
			jsonText: `{
  "extends": "./tsconfig-base.json",
  "compilerOptions": {
    "types": null,
    "lib": null,
    "typeRoots": null
  }
}`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList: map[string]string{
				"/tsconfig-base.json": `{
  "compilerOptions": {
    "types": ["node", "@types/jest"],
    "lib": ["es2020", "dom"],
    "typeRoots": ["./types", "./node_modules/@types"]
  }
}`,
				"/app.ts": "",
			},
		}},
	},
	{
		title:               "null overrides in extended tsconfig - string fields",
		noSubmoduleBaseline: true,
		input: []testConfig{{
			jsonText: `{
  "extends": "./tsconfig-base.json",
  "compilerOptions": {
    "outDir": null,
    "baseUrl": null,
    "rootDir": null
  }
}`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList: map[string]string{
				"/tsconfig-base.json": `{
  "compilerOptions": {
    "outDir": "./dist",
    "baseUrl": "./src",
    "rootDir": "./src"
  }
}`,
				"/app.ts": "",
			},
		}},
	},
	{
		title:               "null overrides in extended tsconfig - mixed field types",
		noSubmoduleBaseline: true,
		input: []testConfig{{
			jsonText: `{
  "extends": "./tsconfig-base.json",
  "compilerOptions": {
    "types": null,
    "outDir": null,
    "strict": false,
    "lib": ["es2022"],
    "allowJs": null
  }
}`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList: map[string]string{
				"/tsconfig-base.json": `{
  "compilerOptions": {
    "types": ["node"],
    "lib": ["es2020", "dom"],
    "outDir": "./dist",
    "strict": true,
    "allowJs": true,
    "target": "es2020"
  }
}`,
				"/app.ts": "",
			},
		}},
	},
	{
		title:               "null overrides with multiple extends levels",
		noSubmoduleBaseline: true,
		input: []testConfig{{
			jsonText: `{
  "extends": "./tsconfig-middle.json",
  "compilerOptions": {
    "types": null,
    "lib": null
  }
}`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList: map[string]string{
				"/tsconfig-middle.json": `{
  "extends": "./tsconfig-base.json",
  "compilerOptions": {
    "types": ["jest"],
    "outDir": "./build"
  }
}`,
				"/tsconfig-base.json": `{
  "compilerOptions": {
    "types": ["node"],
    "lib": ["es2020"],
    "outDir": "./dist",
    "strict": true
  }
}`,
				"/app.ts": "",
			},
		}},
	},
	{
		title:               "null overrides in middle level of extends chain",
		noSubmoduleBaseline: true,
		input: []testConfig{{
			jsonText: `{
  "extends": "./tsconfig-middle.json",
  "compilerOptions": {
    "outDir": "./final"
  }
}`,
			configFileName: "tsconfig.json",
			basePath:       "/",
			allFileList: map[string]string{
				"/tsconfig-middle.json": `{
  "extends": "./tsconfig-base.json",
  "compilerOptions": {
    "types": null,
    "lib": null,
    "outDir": "./middle"
  }
}`,
				"/tsconfig-base.json": `{
  "compilerOptions": {
    "types": ["node"],
    "lib": ["es2020"],
    "outDir": "./base",
    "strict": true
  }
}`,
				"/app.ts": "",
			},
		}},
	},
}

var tsconfigWithExtends = `{
  "files": ["/src/index.ts", "/src/app.ts"],
  "include": ["/src/**/*"],
  "exclude": [],
  "ts-node": {
    "compilerOptions": {
      "module": "commonjs"
    },
    "transpileOnly": true
  }
}`

var tsconfigWithoutConfigDir = `{
  "compilerOptions": {
    "outDir": "bin"
  }
}`

var tsconfigWithConfigDir = `{
  "compilerOptions": {
    "outDir": "${configDir}/bin"
  }
}`

var tsconfigWithExtendsAndConfigDir = `{
  "compilerOptions": {
    "outFile": "${configDir}/outFile",
    "outDir": "${configDir}/outDir",
    "rootDir": "${configDir}/rootDir",
    "tsBuildInfoFile": "${configDir}/tsBuildInfoFile",
    "baseUrl": "${configDir}/baseUrl",
    "declarationDir": "${configDir}/declarationDir",
  }
}`

func TestParseJsonConfigFileContent(t *testing.T) {
	t.Parallel()
	repo.SkipIfNoTypeScriptSubmodule(t)
	for _, rec := range parseJsonConfigFileTests {
		t.Run(rec.title+" with json api", func(t *testing.T) {
			t.Parallel()
			baselineParseConfigWith(t, rec.title+" with json api.js", rec.noSubmoduleBaseline, rec.input, getParsedWithJsonApi)
		})
	}
}

func getParsedWithJsonApi(config testConfig, host tsoptions.ParseConfigHost, basePath string) *tsoptions.ParsedCommandLine {
	configFileName := tspath.GetNormalizedAbsolutePath(config.configFileName, basePath)
	path := tspath.ToPath(config.configFileName, basePath, host.FS().UseCaseSensitiveFileNames())
	parsed, _ := tsoptions.ParseConfigFileTextToJson(configFileName, path, config.jsonText)
	return tsoptions.ParseJsonConfigFileContent(
		parsed,
		host,
		basePath,
		nil,
		configFileName,
		/*resolutionStack*/ nil,
		/*extraFileExtensions*/ nil,
		/*extendedConfigCache*/ nil,
	)
}

func TestParseJsonSourceFileConfigFileContent(t *testing.T) {
	t.Parallel()
	repo.SkipIfNoTypeScriptSubmodule(t)
	for _, rec := range parseJsonConfigFileTests {
		t.Run(rec.title+" with jsonSourceFile api", func(t *testing.T) {
			t.Parallel()
			baselineParseConfigWith(t, rec.title+" with jsonSourceFile api.js", rec.noSubmoduleBaseline, rec.input, getParsedWithJsonSourceFileApi)
		})
	}
}

func getParsedWithJsonSourceFileApi(config testConfig, host tsoptions.ParseConfigHost, basePath string) *tsoptions.ParsedCommandLine {
	configFileName := tspath.GetNormalizedAbsolutePath(config.configFileName, basePath)
	path := tspath.ToPath(config.configFileName, basePath, host.FS().UseCaseSensitiveFileNames())
	parsed := parser.ParseSourceFile(ast.SourceFileParseOptions{
		FileName: configFileName,
		Path:     path,
	}, config.jsonText, core.ScriptKindJSON)
	tsConfigSourceFile := &tsoptions.TsConfigSourceFile{
		SourceFile: parsed,
	}
	return tsoptions.ParseJsonSourceFileConfigFileContent(
		tsConfigSourceFile,
		host,
		host.GetCurrentDirectory(),
		nil,
		configFileName,
		/*resolutionStack*/ nil,
		/*extraFileExtensions*/ nil,
		/*extendedConfigCache*/ nil,
	)
}

func baselineParseConfigWith(t *testing.T, baselineFileName string, noSubmoduleBaseline bool, input []testConfig, getParsed func(config testConfig, host tsoptions.ParseConfigHost, basePath string) *tsoptions.ParsedCommandLine) {
	noSubmoduleBaseline = true
	var baselineContent strings.Builder
	for i, config := range input {
		basePath := config.basePath
		if basePath == "" {
			basePath = tspath.GetNormalizedAbsolutePath(tspath.GetDirectoryPath(config.configFileName), "")
		}
		configFileName := tspath.CombinePaths(basePath, config.configFileName)
		allFileLists := make(map[string]string, len(config.allFileList)+1)
		for file, content := range config.allFileList {
			allFileLists[file] = content
		}
		allFileLists[configFileName] = config.jsonText
		host := tsoptionstest.NewVFSParseConfigHost(allFileLists, config.basePath, true /*useCaseSensitiveFileNames*/)
		parsedConfigFileContent := getParsed(config, host, basePath)

		baselineContent.WriteString("Fs::\n")
		if err := printFS(&baselineContent, host.FS(), "/"); err != nil {
			t.Fatal(err)
		}
		baselineContent.WriteString("\n")
		baselineContent.WriteString("configFileName:: " + config.configFileName + "\n")
		if noSubmoduleBaseline {
			baselineContent.WriteString("CompilerOptions::\n")
			assert.NilError(t, jsonutil.MarshalIndentWrite(&baselineContent, parsedConfigFileContent.ParsedConfig.CompilerOptions, "", "  "))
			baselineContent.WriteString("\n")
			baselineContent.WriteString("\n")

			if parsedConfigFileContent.ParsedConfig.TypeAcquisition != nil {
				baselineContent.WriteString("TypeAcquisition::\n")
				assert.NilError(t, jsonutil.MarshalIndentWrite(&baselineContent, parsedConfigFileContent.ParsedConfig.TypeAcquisition, "", "  "))
				baselineContent.WriteString("\n")
				baselineContent.WriteString("\n")
			}
		}
		baselineContent.WriteString("FileNames::\n")
		baselineContent.WriteString(strings.Join(parsedConfigFileContent.ParsedConfig.FileNames, ",") + "\n")
		baselineContent.WriteString("Errors::\n")
		diagnosticwriter.FormatDiagnosticsWithColorAndContext(&baselineContent, parsedConfigFileContent.Errors, &diagnosticwriter.FormattingOptions{
			NewLine: "\r\n",
			ComparePathsOptions: tspath.ComparePathsOptions{
				CurrentDirectory:          basePath,
				UseCaseSensitiveFileNames: true,
			},
		})
		baselineContent.WriteString("\n")
		if i != len(input)-1 {
			baselineContent.WriteString("\n")
		}
	}
	if noSubmoduleBaseline {
		baseline.Run(t, baselineFileName, baselineContent.String(), baseline.Options{Subfolder: "config/tsconfigParsing"})
	} else {
		baseline.RunAgainstSubmodule(t, baselineFileName, baselineContent.String(), baseline.Options{Subfolder: "config/tsconfigParsing"})
	}
}

func writeJsonReadableText(output io.Writer, input any) error {
	return jsonutil.MarshalIndentWrite(output, input, "", "  ")
}

func TestParseTypeAcquisition(t *testing.T) {
	t.Parallel()
	// repo.SkipIfNoTypeScriptSubmodule(t)
	cases := []struct {
		title      string
		configName string
		config     string
	}{
		{
			title: "Convert correctly format tsconfig.json to typeAcquisition ",
			config: `{
	"typeAcquisition": {
		"enable": true,
		"include": ["0.d.ts", "1.d.ts"],
		"exclude": ["0.js", "1.js"],
	},
}`,
			configName: "tsconfig.json",
		},
		{
			title: "Convert incorrect format tsconfig.json to typeAcquisition ",
			config: `{
	"typeAcquisition": {
		"enableAutoDiscovy": true,
	}
}`, configName: "tsconfig.json",
		},
		{
			title:  "Convert default tsconfig.json to typeAcquisition ",
			config: `{}`, configName: "tsconfig.json",
		},
		{
			title: "Convert tsconfig.json with only enable property to typeAcquisition ",
			config: `{
	"typeAcquisition": {
		"enable": true,
	},
}`, configName: "tsconfig.json",
		},

		// jsconfig.json
		{
			title: "Convert jsconfig.json to typeAcquisition ",
			config: `{
	"typeAcquisition": {
		"enable": false,
		"include": ["0.d.ts"],
		"exclude": ["0.js"],
	},
}`,
			configName: "jsconfig.json",
		},
		{title: "Convert default jsconfig.json to typeAcquisition ", config: `{}`, configName: "jsconfig.json"},
		{
			title: "Convert incorrect format jsconfig.json to typeAcquisition ",
			config: `{
	"typeAcquisition": {
		"enableAutoDiscovy": true,
	},
}`,
			configName: "jsconfig.json",
		},
		{
			title: "Convert jsconfig.json with only enable property to typeAcquisition ",
			config: `{
	"typeAcquisition": {
		"enable": false,
	},
}`,
			configName: "jsconfig.json",
		},
	}
	for _, test := range cases {
		withJsonApiName := test.title + " with json api"
		input := []testConfig{
			{
				jsonText:       test.config,
				configFileName: test.configName,
				basePath:       "/apath",
				allFileList: map[string]string{
					"/apath/a.ts": "",
					"/apath/b.ts": "",
				},
			},
		}
		t.Run(withJsonApiName, func(t *testing.T) {
			t.Parallel()
			baselineParseConfigWith(t, withJsonApiName+".js", true, input, getParsedWithJsonApi)
		})
		withJsonSourceFileApiName := test.title + " with jsonSourceFile api"
		t.Run(withJsonSourceFileApiName, func(t *testing.T) {
			t.Parallel()
			baselineParseConfigWith(t, withJsonSourceFileApiName+".js", true, input, getParsedWithJsonSourceFileApi)
		})
	}
}

func printFS(output io.Writer, files vfs.FS, root string) error {
	return files.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			if content, ok := files.ReadFile(path); !ok {
				return fmt.Errorf("failed to read file %s", path)
			} else {
				if _, err := fmt.Fprintf(output, "//// [%s]\r\n%s\r\n\r\n", path, content); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func TestParseSrcCompiler(t *testing.T) {
	t.Parallel()

	repo.SkipIfNoTypeScriptSubmodule(t)

	compilerDir := tspath.NormalizeSlashes(filepath.Join(repo.TypeScriptSubmodulePath, "src", "compiler"))
	tsconfigFileName := tspath.CombinePaths(compilerDir, "tsconfig.json")

	fs := osvfs.FS()
	host := &tsoptionstest.VfsParseConfigHost{
		Vfs:              fs,
		CurrentDirectory: compilerDir,
	}

	jsonText, ok := fs.ReadFile(tsconfigFileName)
	assert.Assert(t, ok)
	tsconfigPath := tspath.ToPath(tsconfigFileName, compilerDir, fs.UseCaseSensitiveFileNames())
	parsed := parser.ParseSourceFile(ast.SourceFileParseOptions{
		FileName: tsconfigFileName,
		Path:     tsconfigPath,
	}, jsonText, core.ScriptKindJSON)

	if len(parsed.Diagnostics()) > 0 {
		for _, error := range parsed.Diagnostics() {
			t.Log(error.Message())
		}
		t.FailNow()
	}

	tsConfigSourceFile := &tsoptions.TsConfigSourceFile{
		SourceFile: parsed,
	}

	parseConfigFileContent := tsoptions.ParseJsonSourceFileConfigFileContent(
		tsConfigSourceFile,
		host,
		host.GetCurrentDirectory(),
		nil,
		tsconfigFileName,
		/*resolutionStack*/ nil,
		/*extraFileExtensions*/ nil,
		/*extendedConfigCache*/ nil,
	)

	if len(parseConfigFileContent.Errors) > 0 {
		for _, error := range parseConfigFileContent.Errors {
			t.Log(error.Message())
		}
		t.FailNow()
	}

	opts := parseConfigFileContent.CompilerOptions()
	assert.DeepEqual(t, opts, &core.CompilerOptions{
		Lib:                        []string{"lib.es2020.d.ts"},
		Module:                     core.ModuleKindNodeNext,
		ModuleResolution:           core.ModuleResolutionKindNodeNext,
		NewLine:                    core.NewLineKindLF,
		OutDir:                     tspath.NormalizeSlashes(filepath.Join(repo.TypeScriptSubmodulePath, "built", "local")),
		Target:                     core.ScriptTargetES2020,
		Types:                      []string{"node"},
		ConfigFilePath:             tsconfigFileName,
		Declaration:                core.TSTrue,
		DeclarationMap:             core.TSTrue,
		EmitDeclarationOnly:        core.TSTrue,
		AlwaysStrict:               core.TSTrue,
		Composite:                  core.TSTrue,
		IsolatedDeclarations:       core.TSTrue,
		NoImplicitOverride:         core.TSTrue,
		PreserveConstEnums:         core.TSTrue,
		RootDir:                    tspath.NormalizeSlashes(filepath.Join(repo.TypeScriptSubmodulePath, "src")),
		SkipLibCheck:               core.TSTrue,
		Strict:                     core.TSTrue,
		StrictBindCallApply:        core.TSFalse,
		SourceMap:                  core.TSTrue,
		UseUnknownInCatchVariables: core.TSFalse,
		Pretty:                     core.TSTrue,
	}, cmpopts.IgnoreUnexported(core.CompilerOptions{}))

	fileNames := parseConfigFileContent.ParsedConfig.FileNames
	relativePaths := make([]string, 0, len(fileNames))
	for _, fileName := range fileNames {
		if strings.Contains(fileName, ".generated.") {
			continue
		}

		relativePaths = append(relativePaths, tspath.ConvertToRelativePath(fileName, tspath.ComparePathsOptions{
			CurrentDirectory:          compilerDir,
			UseCaseSensitiveFileNames: fs.UseCaseSensitiveFileNames(),
		}))
	}

	assert.DeepEqual(t, relativePaths, []string{
		"binder.ts",
		"builder.ts",
		"builderPublic.ts",
		"builderState.ts",
		"builderStatePublic.ts",
		"checker.ts",
		"commandLineParser.ts",
		"core.ts",
		"corePublic.ts",
		"debug.ts",
		"emitter.ts",
		"executeCommandLine.ts",
		"expressionToTypeNode.ts",
		"moduleNameResolver.ts",
		"moduleSpecifiers.ts",
		"parser.ts",
		"path.ts",
		"performance.ts",
		"performanceCore.ts",
		"program.ts",
		"programDiagnostics.ts",
		"resolutionCache.ts",
		"scanner.ts",
		"semver.ts",
		"sourcemap.ts",
		"symbolWalker.ts",
		"sys.ts",
		"tracing.ts",
		"transformer.ts",
		"tsbuild.ts",
		"tsbuildPublic.ts",
		"types.ts",
		"utilities.ts",
		"utilitiesPublic.ts",
		"visitorPublic.ts",
		"watch.ts",
		"watchPublic.ts",
		"watchUtilities.ts",
		"_namespaces/ts.moduleSpecifiers.ts",
		"_namespaces/ts.performance.ts",
		"_namespaces/ts.ts",
		"factory/baseNodeFactory.ts",
		"factory/emitHelpers.ts",
		"factory/emitNode.ts",
		"factory/nodeChildren.ts",
		"factory/nodeConverters.ts",
		"factory/nodeFactory.ts",
		"factory/nodeTests.ts",
		"factory/parenthesizerRules.ts",
		"factory/utilities.ts",
		"factory/utilitiesPublic.ts",
		"transformers/classFields.ts",
		"transformers/classThis.ts",
		"transformers/declarations.ts",
		"transformers/destructuring.ts",
		"transformers/es2016.ts",
		"transformers/es2017.ts",
		"transformers/es2018.ts",
		"transformers/es2019.ts",
		"transformers/es2020.ts",
		"transformers/es2021.ts",
		"transformers/esDecorators.ts",
		"transformers/esnext.ts",
		"transformers/jsx.ts",
		"transformers/legacyDecorators.ts",
		"transformers/namedEvaluation.ts",
		"transformers/taggedTemplate.ts",
		"transformers/ts.ts",
		"transformers/typeSerializer.ts",
		"transformers/utilities.ts",
		"transformers/declarations/diagnostics.ts",
		"transformers/module/esnextAnd2015.ts",
		"transformers/module/impliedNodeFormatDependent.ts",
		"transformers/module/module.ts",
		"transformers/module/system.ts",
	})
}

func BenchmarkParseSrcCompiler(b *testing.B) {
	repo.SkipIfNoTypeScriptSubmodule(b)

	compilerDir := tspath.NormalizeSlashes(filepath.Join(repo.TypeScriptSubmodulePath, "src", "compiler"))
	tsconfigFileName := tspath.CombinePaths(compilerDir, "tsconfig.json")

	fs := osvfs.FS()
	host := &tsoptionstest.VfsParseConfigHost{
		Vfs:              fs,
		CurrentDirectory: compilerDir,
	}

	jsonText, ok := fs.ReadFile(tsconfigFileName)
	assert.Assert(b, ok)
	tsconfigPath := tspath.ToPath(tsconfigFileName, compilerDir, fs.UseCaseSensitiveFileNames())
	parsed := parser.ParseSourceFile(ast.SourceFileParseOptions{
		FileName: tsconfigFileName,
		Path:     tsconfigPath,
	}, jsonText, core.ScriptKindJSON)

	b.ReportAllocs()

	for b.Loop() {
		tsoptions.ParseJsonSourceFileConfigFileContent(
			&tsoptions.TsConfigSourceFile{
				SourceFile: parsed,
			},
			host,
			host.GetCurrentDirectory(),
			nil,
			tsconfigFileName,
			/*resolutionStack*/ nil,
			/*extraFileExtensions*/ nil,
			/*extendedConfigCache*/ nil,
		)
	}
}
