// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bytesx

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	logrusx "github.com/sighupio/furyctl/internal/x/logrus"
)

type TransformFunc func([]byte) ([]byte, error)

// This constant was taken from the following public repository:
// Name: stripansi
// URL: https://github.com/acarl005/stripansi
// Commit: 5a71ef0e047df0427e87a79f27009029921f1f9b
// Author: https://github.com/acarl005
// License: MIT License.

const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))" //nolint:lll // Cannot split regex in multiple lines.

var (
	ErrJSONTransform = errors.New("error while transform to json")
	reg              = regexp.MustCompile(ansi)
)

func StripColor(p []byte) ([]byte, error) {
	s := string(p)

	strippedS := reg.ReplaceAllString(s, "")

	return []byte(strippedS), nil
}

func ToJSONLogFormat(level, action string) TransformFunc {
	timestamp := time.Now().Format(time.RFC3339)

	return func(p []byte) ([]byte, error) {
		var a *string

		msg := string(p)

		if action != "" {
			a = &action
		}

		lf := logrusx.LogFormat{
			Level:  level,
			Action: a,
			Msg:    msg,
			Time:   timestamp,
		}

		out, err := json.Marshal(lf)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrJSONTransform, err)
		}

		return out, nil
	}
}

func Identity(b []byte) ([]byte, error) {
	return b, nil
}
