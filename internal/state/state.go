// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/rs/xid"

	iox "github.com/sighupio/furyctl/internal/x/io"
)

type State struct {
	ID string `json:"id"`
}

func (s *State) WriteFile(name string) error {
	stateBytes, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("error while writing state file: %w", err)
	}

	if err := os.WriteFile(name, stateBytes, iox.FullRWPermAccess); err != nil {
		return fmt.Errorf("error while writing state file: %w", err)
	}

	return nil
}

func ReadOrCreate(name string) (*State, error) {
	_, err := os.Stat(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			guid := xid.New().String()

			state := State{
				ID: guid,
			}

			if err := state.WriteFile(name); err != nil {
				return nil, fmt.Errorf("error while writing state file: %w", err)
			}

			return &state, nil
		}

		return nil, fmt.Errorf("error while reading state file: %w", err)
	}

	fileBytes, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("error while reading state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(fileBytes, &state); err != nil {
		return nil, fmt.Errorf("error while reading state file: %w", err)
	}

	return &state, nil
}
