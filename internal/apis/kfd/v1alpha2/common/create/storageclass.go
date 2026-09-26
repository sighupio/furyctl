// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/sighupio/furyctl/internal/upgrade"
	"github.com/sighupio/furyctl/pkg/reducers"
)

const (
	// StorageClassWaitTimeout is how long furyctl waits for a default StorageClass before it runs
	// the distribution phase again.
	StorageClassWaitTimeout = 2 * time.Minute
	// StorageClassWaitInterval is the time between two checks for a default StorageClass.
	StorageClassWaitInterval = 5 * time.Second
)

// StorageClassLister lists the StorageClasses of the cluster, as kubectl.Runner.Get does.
type StorageClassLister interface {
	Get(sensitive bool, ns string, params ...string) (string, error)
}

// StorageSkipper is a distribution phase that reports the packages it left out because the
// cluster had no default StorageClass.
type StorageSkipper interface {
	SkippedStoragePackages() []string
	WaitForDefaultStorageClass(timeout, interval time.Duration) (string, bool)
	ReportStillSkipped() bool
}

// StorageClassCheck checks for a default StorageClass and remembers, in memory only, the
// packages that the last render of the distribution phase left out.
type StorageClassCheck struct {
	lister  StorageClassLister
	dryRun  bool
	skipped []string
	// The default StorageClass that WaitForDefaultStorageClass found. The next check uses it once
	// instead of asking the cluster again.
	found string
	// True when ReportStillSkipped already logged the skipped packages of the last check.
	reported bool
}

func NewStorageClassCheck(lister StorageClassLister, dryRun bool) *StorageClassCheck {
	return &StorageClassCheck{lister: lister, dryRun: dryRun}
}

// CheckDefaultStorageClass returns the value of checks.storageClassAvailable for the
// templates, and records the packages that the templates skip without it.
func (s *StorageClassCheck) CheckDefaultStorageClass(data map[string]map[any]any) (bool, error) {
	if s.found != "" {
		logrus.Debugf("Using the default StorageClass %q found while waiting", s.found)

		s.found = ""
		s.skipped = nil

		return true, nil
	}

	logrus.Info("Checking for a default storage class...")

	out, err := s.lister.Get(false, "", "storageclasses")
	if err != nil {
		return false, fmt.Errorf("error while checking storage class: %w", err)
	}

	if defaultStorageClassName(out) != "" {
		s.skipped = nil

		return true, nil
	}

	s.skipped = skippedStoragePackages(data)
	s.reported = false

	missing := "No *default* StorageClass found in the cluster"
	if out == "No resources found" {
		missing = "No StorageClass found in the cluster"
	}

	if len(s.skipped) == 0 {
		logrus.Debugf("%s. No enabled package needs one.", missing)

		return false, nil
	}

	logrus.Warnf(
		"%s. furyctl skips these packages: %s. "+
			"furyctl checks again later in this apply and, if a default StorageClass exists by then, applies "+
			"the distribution phase again to install them. If not, install a default StorageClass and run "+
			"furyctl again.",
		missing,
		strings.Join(s.skipped, ", "),
	)

	return false, nil
}

// SkippedStoragePackages returns the packages that the last render left out.
func (s *StorageClassCheck) SkippedStoragePackages() []string {
	return s.skipped
}

// ReportStillSkipped logs that the skipped packages stay skipped. It logs at info level one time
// for each check that skipped packages, and at debug level after that, so the two check points
// of an apply do not log the same line two times. It reports whether it logged at info level.
func (s *StorageClassCheck) ReportStillSkipped() bool {
	msg := "No default StorageClass yet, these packages stay skipped: " + strings.Join(s.skipped, ", ")

	if s.reported {
		logrus.Debug(msg)

		return false
	}

	s.reported = true

	logrus.Info(msg)

	return true
}

// WaitForDefaultStorageClass polls the cluster for a default StorageClass until the timeout,
// and returns its name. In dry-run mode, it does not wait.
func (s *StorageClassCheck) WaitForDefaultStorageClass(timeout, interval time.Duration) (string, bool) {
	if s.dryRun {
		if timeout == 0 {
			logrus.Info("Dry-run mode: a real apply checks here for a default StorageClass and, " +
				"if one exists, applies the distribution phase again")
		} else {
			logrus.Infof("Dry-run mode: a real apply waits up to %s for a default StorageClass and, "+
				"if one appears, applies the distribution phase again", timeout)
		}

		return "", false
	}

	deadline := time.Now().Add(timeout)

	for polls := 0; ; polls++ {
		out, err := s.lister.Get(false, "", "storageclasses")
		if err != nil {
			logrus.Debugf("error while checking storage class: %v", err)
		} else if name := defaultStorageClassName(out); name != "" {
			s.found = name

			return name, true
		}

		if time.Now().Add(interval).After(deadline) {
			return "", false
		}

		// Log the wait only when the StorageClass is not there at once.
		if polls == 0 {
			logrus.Infof("Waiting up to %s for a default StorageClass...", timeout)
		}

		time.Sleep(interval)
	}
}

// PluginsStorageClassWait is the wait for ReapplyDistribution after the plugins phase: the
// StorageClassWaitTimeout when the plugins phase applied plugins, otherwise 0.
func PluginsStorageClassWait(plugins interface{ Applied() bool }) time.Duration {
	if plugins.Applied() {
		return StorageClassWaitTimeout
	}

	return 0
}

// ReapplyDistribution runs the distribution phase again when its last run left out the
// storage-backed packages and a default StorageClass is there within the wait. A wait of 0 checks
// one time: after the distribution phase, kapp already waited for the customResources, so a
// provider there already made its StorageClass. After the plugins phase, the plugins do not wait
// for their pods, so the creator passes StorageClassWaitTimeout.
// A run with the StorageClass available clears the skipped packages, so it runs one time at most.
// Pass the phase as the creator runs it (with its decorators), and the undecorated phase as the
// skipper: the upgrade decorator does not expose StorageSkipper.
func ReapplyDistribution(
	phase upgrade.ReducersOperatorPhase[reducers.Reducers],
	skipper StorageSkipper,
	wait time.Duration,
	upgr *upgrade.Upgrade,
	upgradeState *upgrade.State,
) error {
	skipped := skipper.SkippedStoragePackages()
	if len(skipped) == 0 {
		return nil
	}

	name, found := skipper.WaitForDefaultStorageClass(wait, StorageClassWaitInterval)
	if !found {
		skipper.ReportStillSkipped()

		return nil
	}

	logrus.Infof(
		"A default StorageClass is now available (%q). "+
			"Applying the distribution phase again to install the packages that were skipped: %s",
		name,
		strings.Join(skipped, ", "),
	)

	// Same as --post-apply-phases distribution: no upgrade scripts and no reducers.
	initialUpgrade := upgr.Enabled

	phase.SetUpgrade(false)
	defer phase.SetUpgrade(initialUpgrade)

	if err := phase.Exec(nil, "", upgradeState); err != nil {
		return fmt.Errorf("error while applying the distribution phase again: %w", err)
	}

	return nil
}

// defaultStorageClassName returns the name of the default StorageClass in the output of
// `kubectl get storageclasses`, or "".
func defaultStorageClassName(out string) string {
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "(default)" {
			return fields[0]
		}
	}

	return ""
}

// skippedStoragePackages returns the enabled packages that the distribution templates skip when
// checks.storageClassAvailable is false. Keep it in sync with the storageClassAvailable conditions
// of templates/distribution/manifests/kustomization.yaml.tpl and
// manifests/monitoring/kustomization.yaml.tpl in the distribution repository.
func skippedStoragePackages(data map[string]map[any]any) []string {
	distro, ok := data["spec"]["distribution"].(map[any]any)
	if !ok {
		return nil
	}

	modules, ok := distro["modules"].(map[any]any)
	if !ok {
		return nil
	}

	moduleType := func(name string) string {
		module, ok := modules[name].(map[any]any)
		if !ok {
			return ""
		}

		t, ok := module["type"].(string)
		if !ok {
			return ""
		}

		return t
	}

	var skipped []string

	for _, name := range []string{"dr", "logging"} {
		if t := moduleType(name); t != "" && t != "none" {
			skipped = append(skipped, name)
		}
	}

	if moduleType("tracing") == "tempo" {
		skipped = append(skipped, "tracing")
	}

	if t := moduleType("monitoring"); t == "prometheus" {
		skipped = append(skipped, "prometheus-operated")
	} else if t == "mimir" {
		skipped = append(skipped, "mimir")
	}

	return skipped
}
