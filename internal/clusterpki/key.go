// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package clusterpki

import (
	"crypto"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/keyutil"
)

func EncodePrivateKey(key crypto.PrivateKey) []byte {
	mKey, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		// This is an unmanaged error, we should os.Exit() here but the linter does not let us.
		logrus.Error("error while encoding private key: ", err)
	}

	return mKey
}
