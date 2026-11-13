// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"fmt"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"
	"go.yaml.in/yaml/v3"
)

const helmfileDeleteTimeout = "900"

// KubectlRunner is the part of kubectl.Runner that the plugins delete uses.
type KubectlRunner interface {
	Delete(params ...string) error
	Get(sensitive bool, ns string, params ...string) (string, error)
}

// HelmfileRunner is the part of helmfile.Runner that the plugins delete uses.
type HelmfileRunner interface {
	Destroy(params ...string) error
}

type release struct {
	name, namespace, chart, version string
}

// deleteRelease ends the custom resources of a release while its controllers run, then destroys it.
func (p *Plugins) deleteRelease(r release) error {
	crs, crds, err := p.releaseObjects(r)
	if err != nil {
		return err
	}

	if p.dryRun {
		logrus.Infof("Dry run: helm release %s would be destroyed after these objects:", r.name)

		for _, o := range append(crs, crds...) {
			logrus.Infof("Dry run: %s %s would be deleted", o.Kind, o.ref())
		}

		return nil
	}

	logrus.Infof("Deleting helm release %s...", r.name)

	// Helm uninstall does not wait for finalizers: custom resources and CRDs go first, while the controllers run.
	for _, o := range append(crs, crds...) {
		if err := p.deleteAndWait(o); err != nil {
			return err
		}
	}

	if err := p.destroy(r); err != nil {
		return err
	}

	p.reportPVCs(r)

	return nil
}

// deleteAndWait deletes an object within the grace period; one left only on a finalizer is reported.
func (p *Plugins) deleteAndWait(o object) error {
	args := []string{kubectlResource(o), o.Metadata.Name}
	if o.Metadata.Namespace != "" {
		args = append(args, "-n", o.Metadata.Namespace)
	}

	err := p.runners.Kubectl.Delete(append(args, "--wait", "--timeout="+graceFlag)...)
	if err == nil {
		return nil
	}

	out, gerr := p.runners.Kubectl.Get(true, o.Metadata.Namespace, kubectlResource(o), o.Metadata.Name,
		"--ignore-not-found", "-o", "yaml")
	if gerr != nil {
		return unreadable(err, gerr)
	}

	var left object
	if yerr := yaml.Unmarshal([]byte(out), &left); yerr != nil {
		return unreadable(err, yerr)
	}

	switch {
	case left.Kind == "":
		return nil
	case !left.stuck():
		return fmt.Errorf("error deleting %s %s: %w", o.Kind, o.ref(), err)
	default:
		reportStuck([]object{left})

		return nil
	}
}

// kubectlResource names the kind with its group, as kubectl resolves it: Bucket.example.com.
func kubectlResource(o object) string {
	if o.group() == "" {
		return o.Kind
	}

	return o.Kind + "." + o.group()
}

func (p *Plugins) destroy(r release) error {
	params := []string{
		"--selector", "name=" + r.name,
		"--deleteWait", "--deleteTimeout", helmfileDeleteTimeout,
		"--cascade", "foreground",
	}

	err := p.runners.Helmfile.Destroy(params...)
	if err == nil {
		return nil
	}

	logrus.Warnf("helmfile destroy of release %s failed, retrying without hooks: %v", r.name, err)

	noHooks := append(slices.Clone(params), "--args", "--no-hooks")
	if err := p.runners.Helmfile.Destroy(noHooks...); err != nil {
		return fmt.Errorf("error destroying helm release %s: %w", r.name, err)
	}

	return nil
}

// reportPVCs names the PVCs left in the release namespace: their data is not deleted without the operator.
func (p *Plugins) reportPVCs(r release) {
	if r.namespace == "" {
		return
	}

	out, err := p.runners.Kubectl.Get(false, r.namespace, "persistentvolumeclaims",
		"-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		logrus.Warnf("Cannot list the PVCs left in namespace %s: %v", r.namespace, err)

		return
	}

	for pvc := range strings.FieldsSeq(out) {
		logrus.Warnf("PersistentVolumeClaim %s/%s is left after helm release %s: delete it when its data is not needed",
			r.namespace, pvc, r.name)
	}
}
