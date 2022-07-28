#!/usr/bin/env bats
# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.


load "./../../helper"

OS="linux"
if [[ "${OSTYPE}" == "darwin"* ]]; then
    OS="darwin"
fi
CPUARCH="amd64_v1"
if [ "$(uname -m)" = "arm64" ]; then
	CPUARCH="arm64"
fi

@test "furyctl" {
    info
    init(){
        ./dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl version
    }
    run init
    if [[ ${status} -ne 0 ]]; then
        echo "${output}" >&3
    fi
    [ "${status}" -eq 0 ]
}

@test "template simple-dry-run" {
    info
    test_dir="./automated-tests/integration/template-engine/test-data/simple-dry-run"
    init(){
        cd ${test_dir} && ../../../../../dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl -d --debug template --dry-run
        cat ./target/file.txt | grep "testValue"
    }
    run init

    if [[ ${status} -ne 0 ]]; then
        echo "${output}" >&3
    fi
    [ "${status}" -eq 0 ]
}

@test "template simple" {
    info
    test_dir="./automated-tests/integration/template-engine/test-data/simple"
    init(){
        cd ${test_dir} && ../../../../../dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl -d --debug template
        cat ./target/file.txt | grep "testValue"
    }
    run init

    if [[ ${status} -ne 0 ]]; then
        echo "${output}" >&3
    fi
    [ "${status}" -eq 0 ]
}
