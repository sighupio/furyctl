// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package create

import (
	"github.com/sirupsen/logrus"
)

// warnSecretFiles tells the user to handle the files with care. `create pki` and `create secrets`
// both write private keys and passwords in clear text.
func warnSecretFiles(folder string) {
	logrus.Warnf(
		"The files in %s hold private keys and passwords in clear text. Give access to these files "+
			"only to the persons that must use them. Keep a safe copy of them. A cluster that runs "+
			"with these values cannot get new ones without a change to its configuration. If you put "+
			"these files in a git repository, encrypt them first, for example with git-crypt or SOPS.",
		folder,
	)
}
