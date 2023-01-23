// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bytesx

import (
	"bytes"
	"fmt"

	"github.com/sighupio/furyctl/internal/template/mapper"
)

func SafeWriteToBuffer(buffer *bytes.Buffer, content string, values ...any) error {
	vs := make([]any, 0)

	for _, sv := range values {
		if sv == nil {
			continue
		}

		v, err := mapper.ParseDynamicValue(sv)
		if err != nil {
			return fmt.Errorf("error parsing dynamic value: %w", err)
		}

		vs = append(vs, fmt.Sprintf("%v", v))
	}

	if len(vs) == 0 {
		_, err := buffer.WriteString(content)
		if err != nil {
			return fmt.Errorf("error writing to buffer: %w", err)
		}

		return nil
	}

	_, err := buffer.WriteString(fmt.Sprintf(content, vs...))
	if err != nil {
		return fmt.Errorf("error writing to buffer: %w", err)
	}

	return nil
}
