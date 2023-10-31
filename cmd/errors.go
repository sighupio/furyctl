package cmd

import "errors"

var (
	ErrParsingFlag        = errors.New("error while parsing flag")
	ErrKubeconfigReq      = errors.New("either the KUBECONFIG environment variable or the --kubeconfig flag should be set")
	ErrKubeconfigNotFound = errors.New("kubeconfig file not found")
)
