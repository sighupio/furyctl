package app

import (
	"encoding/json"
	"net/http"
)

type Update struct {
	FuryctlBinVersion string
}

type Release struct {
	URL     string `json:"html_url"`
	Version string `json:"name"`
}

func NewUpdate(furyctlBinVersion string) *Update {
	return &Update{
		FuryctlBinVersion: furyctlBinVersion,
	}
}

func (u *Update) FetchLastRelease() (Release, error) {
	var release Release

	resp, err := http.Get("https://api.github.com/repos/sighupio/furyctl/releases/latest")
	if err != nil {
		return release, err
	}

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return release, err
	}

	return release, nil
}
