package packagejson

import (
	"sync"

	"github.com/microsoft/typescript-go/internal/collections"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/diagnostics"
	"github.com/microsoft/typescript-go/internal/semver"
	"github.com/microsoft/typescript-go/internal/tspath"
)

var typeScriptVersion = semver.MustParse(core.Version())

type PackageJson struct {
	Fields
	versionPaths VersionPaths
	once         sync.Once
}

func (p *PackageJson) GetVersionPaths(trace func(string)) VersionPaths {
	p.once.Do(func() {
		if p.Fields.TypesVersions.Type == JSONValueTypeNotPresent {
			if trace != nil {
				trace(diagnostics.X_package_json_does_not_have_a_0_field.Format("typesVersions"))
			}
			return
		}
		if p.Fields.TypesVersions.Type != JSONValueTypeObject {
			if trace != nil {
				trace(diagnostics.Expected_type_of_0_field_in_package_json_to_be_1_got_2.Format("typesVersions", "object", p.Fields.TypesVersions.Type.String()))
			}
			return
		}

		if trace != nil {
			trace(diagnostics.X_package_json_has_a_typesVersions_field_with_version_specific_path_mappings.Format("typesVersions"))
		}

		for key, value := range p.Fields.TypesVersions.AsObject().Entries() {
			keyRange, ok := semver.TryParseVersionRange(key)
			if !ok {
				if trace != nil {
					trace(diagnostics.X_package_json_has_a_typesVersions_entry_0_that_is_not_a_valid_semver_range.Format(key))
				}
				continue
			}
			if keyRange.Test(&typeScriptVersion) {
				if value.Type != JSONValueTypeObject {
					if trace != nil {
						trace(diagnostics.Expected_type_of_0_field_in_package_json_to_be_1_got_2.Format("typesVersions['"+key+"']", "object", value.Type.String()))
					}
					return
				}
				p.versionPaths = VersionPaths{
					Version:   key,
					pathsJSON: value.AsObject(),
				}
				return
			}
		}

		if trace != nil {
			trace(diagnostics.X_package_json_does_not_have_a_typesVersions_entry_that_matches_version_0.Format(core.VersionMajorMinor()))
		}
	})
	return p.versionPaths
}

type VersionPaths struct {
	Version   string
	pathsJSON *collections.OrderedMap[string, JSONValue]
	paths     *collections.OrderedMap[string, []string]
}

func (v *VersionPaths) Exists() bool {
	return v != nil && v.Version != "" && v.pathsJSON != nil
}

func (v *VersionPaths) GetPaths() *collections.OrderedMap[string, []string] {
	if !v.Exists() {
		return nil
	}
	if v.paths != nil {
		return v.paths
	}
	paths := collections.NewOrderedMapWithSizeHint[string, []string](v.pathsJSON.Size())
	for key, value := range v.pathsJSON.Entries() {
		if value.Type != JSONValueTypeArray {
			continue
		}
		slice := make([]string, len(value.AsArray()))
		for i, path := range value.AsArray() {
			if path.Type != JSONValueTypeString {
				continue
			}
			slice[i] = path.Value.(string)
		}
		paths.Set(key, slice)
	}
	v.paths = paths
	return v.paths
}

type InfoCacheEntry struct {
	PackageDirectory string
	DirectoryExists  bool
	Contents         *PackageJson
}

func (p *InfoCacheEntry) Exists() bool {
	return p != nil && p.Contents != nil
}

func (p *InfoCacheEntry) GetContents() *PackageJson {
	if p == nil || p.Contents == nil {
		return nil
	}
	return p.Contents
}

func (p *InfoCacheEntry) GetDirectory() string {
	if p == nil {
		return ""
	}
	return p.PackageDirectory
}

type InfoCache struct {
	mu                        sync.RWMutex
	IsReadonly                bool
	cache                     map[tspath.Path]InfoCacheEntry
	currentDirectory          string
	useCaseSensitiveFileNames bool
}

func NewInfoCache(currentDirectory string, useCaseSensitiveFileNames bool) *InfoCache {
	return &InfoCache{
		currentDirectory:          currentDirectory,
		useCaseSensitiveFileNames: useCaseSensitiveFileNames,
	}
}

func (p *InfoCache) Get(packageJsonPath string) *InfoCacheEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	key := tspath.ToPath(packageJsonPath, p.currentDirectory, p.useCaseSensitiveFileNames)
	entry, ok := p.cache[key]
	if !ok {
		return nil
	}
	return &entry
}

func (p *InfoCache) Set(packageJsonPath string, info *InfoCacheEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := tspath.ToPath(packageJsonPath, p.currentDirectory, p.useCaseSensitiveFileNames)
	if p.cache == nil {
		p.cache = make(map[tspath.Path]InfoCacheEntry)
	}
	p.cache[key] = *info
}
