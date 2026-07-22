// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package template

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	iox "github.com/sighupio/furyctl/internal/x/io"
	"github.com/sighupio/furyctl/pkg/template/mapper"
	yamlx "github.com/sighupio/furyctl/pkg/x/yaml"
)

// distributionManifestsDir is the subdirectory of the distribution render target
// where the main kustomization.yaml is written. Entries of
// spec.distribution.customPatches.resources are resolved relative to it.
const distributionManifestsDir = "manifests"

var (
	ErrTargetIsNotEmpty = errors.New("target directory is not empty")
	errSourceMustbeSet  = errors.New("source must be set")
	errTargetMustbeSet  = errors.New("target must be set")
	errTemplateNotFound = errors.New("no template found")

	// Tracks custom resource paths that have already triggered an "outside the
	// configuration directory" warning. Templates are rendered several times during
	// a single furyctl run (preflight, preupgrade, the distribution phase, ...), so
	// this keeps the warning to once per path per run.
	//
	//nolint:gochecknoglobals // process-wide dedupe set for a user-facing warning.
	warnedResourcesOutsideConfDir sync.Map
)

type Model struct {
	SourcePath           string
	TargetPath           string
	ConfigPath           string
	OutputPath           string
	FuryctlConfPath      string
	Config               Config
	Suffix               string
	Context              map[string]map[any]any
	FuncMap              FuncMap
	StopIfTargetNotEmpty bool
	DryRun               bool
}

func NewTemplateModel(
	source,
	target,
	configPath,
	outPath,
	furyctlConfPath,
	suffix string,
	stopIfNotEmpty,
	dryRun bool,
) (*Model, error) {
	var model Config

	if len(source) < 1 {
		return nil, errSourceMustbeSet
	}

	if len(target) < 1 {
		return nil, errTargetMustbeSet
	}

	if len(configPath) > 0 {
		readFile, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}

		if err = yaml.Unmarshal(readFile, &model); err != nil {
			return nil, fmt.Errorf("error parsing config file: %w", err)
		}
	}

	funcMap := NewFuncMap()
	funcMap.Add("toYaml", ToYAML)
	funcMap.Add("fromYaml", FromYAML)
	funcMap.Add("hasKeyAny", HasKeyAny)
	funcMap.Add("digAny", DigAny)

	return &Model{
		SourcePath:           source,
		TargetPath:           target,
		ConfigPath:           configPath,
		OutputPath:           outPath,
		FuryctlConfPath:      furyctlConfPath,
		Config:               model,
		Suffix:               suffix,
		FuncMap:              funcMap,
		StopIfTargetNotEmpty: stopIfNotEmpty,
		DryRun:               dryRun,
	}, nil
}

func (tm *Model) Generate() error {
	if tm.StopIfTargetNotEmpty {
		err := iox.CheckDirIsEmpty(tm.TargetPath)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrTargetIsNotEmpty, err)
		}
	}

	if err := os.MkdirAll(tm.TargetPath, os.ModePerm); err != nil {
		return fmt.Errorf("error creating target directory: %w", err)
	}

	context, cErr := tm.generateContext()
	if cErr != nil {
		return cErr
	}

	ctxMapper := mapper.NewMapper(
		context,
		tm.FuryctlConfPath,
	)

	context, err := ctxMapper.MapDynamicValuesAndPaths()
	if err != nil {
		return fmt.Errorf("error mapping dynamic values: %w", err)
	}

	tm.relativizeCustomResources(context)

	tm.Context = context

	if err := filepath.Walk(tm.SourcePath, tm.applyTemplates); err != nil {
		return fmt.Errorf("error applying templates: %w", err)
	}

	return nil
}

func (tm *Model) isExcluded(source string) bool {
	for _, exc := range tm.Config.Templates.Excludes {
		regex := regexp.MustCompile(exc)
		if regex.MatchString(source) {
			return true
		}
	}

	return false
}

func (tm *Model) applyTemplates(
	relSource string,
	info os.FileInfo,
	err error,
) error {
	if tm.isExcluded(relSource) {
		return err
	}

	if info == nil {
		return err
	}

	if info.IsDir() {
		return err
	}

	rel, err := filepath.Rel(tm.SourcePath, relSource)
	if err != nil {
		return fmt.Errorf("error getting relative path: %w", err)
	}

	currentTarget := filepath.Join(tm.TargetPath, rel)

	gen := NewGenerator(
		tm.SourcePath,
		relSource,
		currentTarget,
		tm.Context,
		tm.FuncMap,
		tm.DryRun,
	)

	realTarget, fErr := gen.ProcessFilename(tm)
	if fErr != nil { // Maybe we should fail back to real name instead?
		return fErr
	}

	gen.UpdateTarget(realTarget)

	currentTargetDir := filepath.Dir(realTarget)

	if _, err := os.Stat(currentTargetDir); os.IsNotExist(err) {
		if err := os.MkdirAll(currentTargetDir, os.ModePerm); err != nil {
			return fmt.Errorf("error creating target directory: %w", err)
		}
	}

	if strings.HasSuffix(info.Name(), tm.Suffix) {
		tmpl, err := gen.ProcessTemplate()
		if err != nil {
			return err
		}

		if tmpl == nil {
			return fmt.Errorf("%w for %s", errTemplateNotFound, relSource)
		}

		if tm.DryRun {
			missingKeys := gen.GetMissingKeys(tmpl)

			err := gen.WriteMissingKeysToFile(missingKeys, relSource, tm.OutputPath)
			if err != nil {
				return err
			}
		}

		content, cErr := gen.ProcessFile(tmpl)
		if cErr != nil {
			return fmt.Errorf("%w filePath: %s", cErr, relSource)
		}

		err = iox.CopyBufferToFile(content, realTarget)
		if err != nil {
			return fmt.Errorf("error writing file: %w", err)
		}

		return nil
	}

	err = iox.CopyFile(relSource, realTarget)
	if err != nil {
		return fmt.Errorf("error copying file: %w", err)
	}

	return nil
}

func (tm *Model) generateContext() (map[string]map[any]any, error) {
	context := make(map[string]map[any]any)

	maps.Copy(context, tm.Config.Data)

	for k, v := range tm.Config.Include {
		cPath := filepath.Join(filepath.Dir(tm.ConfigPath), v)

		if filepath.IsAbs(v) {
			cPath = v
		}

		yamlConfig, err := yamlx.FromFileV2[map[any]any](cPath)
		if err != nil {
			return nil, err
		}

		context[k] = yamlConfig
	}

	return context, nil
}

// relativizeCustomResources rewrites spec.distribution.customResources so that
// local filesystem paths are expressed relative to the rendered manifests
// directory, which is what kustomize expects.
//
// By the time this runs, the mapper has already turned user-supplied relative paths
// (e.g. "./foo") into absolute paths anchored at the furyctl config directory.
// Kustomize, however, rejects absolute paths in its resources list and resolves
// relative ones against the kustomization.yaml that declares them. We therefore
// convert each local path into one relative to <TargetPath>/manifests, leaving the
// referenced files in place so kustomize bases can still compose with their own
// relative references (the distribution apply script builds with
// --load-restrictor LoadRestrictionsNone).
//
// Remote resources (git/URL references) and entries that don't point to an existing
// local file or directory are left untouched so kustomize can fetch or resolve them
// itself. Emitting a path relative to the config directory (rather than an absolute
// one) keeps the rendered output deterministic across machines, matching the
// behaviour of relativeVendorPath.
func (tm *Model) relativizeCustomResources(context map[string]map[any]any) {
	resources, ok := customResources(context)
	if !ok {
		return
	}

	manifestsDir := filepath.Join(tm.TargetPath, distributionManifestsDir)
	furyctlConfDir := filepath.Dir(tm.FuryctlConfPath)

	for i, raw := range resources {
		entry, ok := raw.(string)
		if !ok || entry == "" {
			continue
		}

		abs := entry
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(furyctlConfDir, abs)
		}

		if _, err := os.Stat(abs); err != nil {
			// Remote resource (git/URL) or non-existent path: leave it untouched
			// so kustomize can fetch or resolve it.
			continue
		}

		if rel, err := filepath.Rel(furyctlConfDir, abs); err == nil && strings.HasPrefix(rel, "..") {
			if _, warned := warnedResourcesOutsideConfDir.LoadOrStore(abs, struct{}{}); !warned {
				logrus.Warnf(
					"custom resource %q resolves outside the furyctl configuration directory; "+
						"the rendered path may differ between machines. Consider keeping custom "+
						"resources inside the project directory.",
					entry,
				)
			}
		}

		rel, err := filepath.Rel(manifestsDir, abs)
		if err != nil {
			continue
		}

		resources[i] = rel
	}
}

// customResources safely navigates to spec.distribution.customResources in the
// template context and returns the underlying slice (mutable in place) when present.
func customResources(context map[string]map[any]any) ([]any, bool) {
	spec, ok := context["spec"]
	if !ok {
		return nil, false
	}

	distribution, ok := spec["distribution"].(map[any]any)
	if !ok {
		return nil, false
	}

	resources, ok := distribution["customResources"].([]any)
	if !ok {
		return nil, false
	}

	return resources, true
}
