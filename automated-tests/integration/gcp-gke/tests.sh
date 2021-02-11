#!/usr/bin/env bats
# Copyright (c) 2021 SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.


load "./../../helper"

@test "Bootstrap init" {
    info
    init(){
        ./dist/furyctl-linux_linux_amd64/furyctl -d --debug bootstrap init --config ./automated-tests/integration/gcp-gke/bootstrap.yml -w ./automated-tests/integration/gcp-gke/bootstrap --reset
    }
    run init
    [ "$status" -eq 0 ]
}

@test "Cluster init" {
    info
    init(){
        ./dist/furyctl-linux_linux_amd64/furyctl -d --debug cluster init --config ./automated-tests/integration/gcp-gke/cluster.yml -w ./automated-tests/integration/gcp-gke/cluster --reset
    }
    run init
    [ "$status" -eq 0 ]
}
