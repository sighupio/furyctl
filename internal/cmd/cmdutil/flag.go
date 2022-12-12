package cmdutil

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sighupio/furyctl/internal/analytics"
)

var ErrParsingFlag = errors.New("error while parsing flag")

func BoolFlag(cmd *cobra.Command, flagName string, tracker *analytics.Tracker, event analytics.Event) (bool, error) {
	value, err := cmd.Flags().GetBool(flagName)
	if err != nil {
		event.AddErrorMessage(fmt.Errorf("%w: %s", ErrParsingFlag, flagName))
		tracker.Track(event)

		return false, fmt.Errorf("%w: %s", ErrParsingFlag, flagName)
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
