// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package del

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// deleteApp runs kapp delete of the app; after a failure, objects stuck on a finalizer are reported, not an error.
func deleteApp(kapp KappRunner, app string, params []string, skip exclusion) error {
	err := runDelete(kapp, app, params, skip)
	if err == nil {
		return nil
	}

	objs, stuck, _, ierr := leftovers(kapp, app, skip)
	if ierr != nil {
		return unreadable(err, ierr)
	}

	if len(stuck) == 0 {
		return err
	}

	reportStuck(stuck)

	// After a timeout kapp stops the later waves, so one more pass deletes the rest around the stuck objects.
	skip = append(append(exclusion{}, skip...), stuckExclusion(stuck, objs)...)

	if err := runDelete(kapp, app, params, skip); err != nil {
		_, stuckAgain, rest, ierr := leftovers(kapp, app, skip)
		if ierr != nil {
			return unreadable(err, ierr)
		}

		if len(rest) > 0 {
			return err
		}

		reportStuck(stuckAgain)
	}

	logrus.Warnf(
		"App %s is deleted except the objects above, stuck on a finalizer after the %s grace period: "+
			"the infrastructure phase erases them with the disks",
		app, graceFlag,
	)

	return nil
}

func unreadable(err, inspectErr error) error {
	return fmt.Errorf("%w; its objects cannot be read: %w", err, inspectErr)
}

func runDelete(kapp KappRunner, app string, params []string, skip exclusion) error {
	args, err := withExclusion(params, skip)
	if err != nil {
		return err
	}

	//nolint:wrapcheck // kapp.Runner already names the app in its error.
	return kapp.Delete(app, args...)
}

// leftovers returns every object of the app, and the ones outside skip split by deletionTimestamp.
func leftovers(kapp KappRunner, app string, skip exclusion) ([]object, []object, []object, error) {
	// The raw objects have finalizers, kapp inspect --json does not.
	raw, err := kapp.Inspect(app, "--raw")
	if err != nil {
		return nil, nil, nil, err //nolint:wrapcheck // kapp.Runner already names the app in its error.
	}

	objs, err := parseObjects(raw)
	if err != nil {
		return nil, nil, nil, err
	}

	var stuck, rest []object

	for _, o := range objs {
		switch {
		case skip.excludes(o):
		case o.stuck():
			stuck = append(stuck, o)
		default:
			rest = append(rest, o)
		}
	}

	return objs, stuck, rest, nil
}

// stuckExclusion selects the stuck objects and the CRDs of the app that serve them: a CRD delete waits for its objects.
func stuckExclusion(stuck, objs []object) exclusion {
	skip := make(exclusion, 0, len(stuck))

	for _, s := range stuck {
		skip = append(skip, objectSelector(s))

		for _, c := range objs {
			if c.Kind == crdKind && c.served.Names.Kind == s.Kind && c.served.Group == s.group() {
				skip = append(skip, objectSelector(c))
			}
		}
	}

	return skip
}

func reportStuck(stuck []object) {
	for _, o := range stuck {
		logrus.Warnf("%s %s is stuck deleting, finalizers: %s", o.Kind, o.ref(), strings.Join(o.Metadata.Finalizers, ", "))
	}
}
