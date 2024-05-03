// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmdutil

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
)

var ErrParsingFlag = errors.New("error while parsing flag")

const AnyGoGetterFormatStr = "Any format supported by hashicorp/go-getter can be used."

func BoolFlag(cmd *cobra.Command, flagName string, tracker *analytics.Tracker, event analytics.Event) (bool, error) {
	value, err := cmd.Flags().GetBool(flagName)
	if err != nil {
		event.AddErrorMessage(fmt.Errorf("%w: %s", ErrParsingFlag, flagName))
		tracker.Track(event)

		return false, fmt.Errorf("%w: %s", ErrParsingFlag, flagName)
	}

	return value, nil
}

func IntFlag(cmd *cobra.Command, flagName string, tracker *analytics.Tracker, event analytics.Event) (int, error) {
	value, err := cmd.Flags().GetInt(flagName)
	if err != nil {
		event.AddErrorMessage(fmt.Errorf("%w: %s", ErrParsingFlag, flagName))
		tracker.Track(event)

		return 0, fmt.Errorf("%w: %s", ErrParsingFlag, flagName)
	}

	return value, nil
}

func StringFlag(cmd *cobra.Command, flagName string, tracker *analytics.Tracker, event analytics.Event) (string, error) {
	value, err := cmd.Flags().GetString(flagName)
	if err != nil {
		event.AddErrorMessage(fmt.Errorf("%w: %s", ErrParsingFlag, flagName))
		tracker.Track(event)

		return "", fmt.Errorf("%w: %s", ErrParsingFlag, flagName)
	}

	return value, nil
}

func StringFlagOptional(cmd *cobra.Command, flagName string) string {
	value, err := cmd.Flags().GetString(flagName)
	if err != nil {
		return ""
	}

	return value
}

func StringSliceFlag(cmd *cobra.Command, flagName string, tracker *analytics.Tracker, event analytics.Event) ([]string, error) {
	value, err := cmd.Flags().GetStringSlice(flagName)
	if err != nil {
		event.AddErrorMessage(fmt.Errorf("%w: %s", ErrParsingFlag, flagName))
		tracker.Track(event)

		return []string{}, fmt.Errorf("%w: %s", ErrParsingFlag, flagName)
	}

	return value, nil
}
