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

@test "invalid furyctl yaml" {
    info
    test_dir="./automated-tests/integration/validation-cmd/test-data/invalid-furyctl-yaml"
    abs_test_dir=${PWD}/${test_dir}
    furyctl_bin=${PWD}/dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl
    init(){
        cd ${test_dir} && ${furyctl_bin} -d --debug validate config --config ${abs_test_dir}/furyctl.yaml --distro-location ${abs_test_dir}
    }
    run init

    [ "${status}" -eq 1 ]

    if [[ ${output} != *"furyctl.yaml contains validation errors"* ]]; then
        echo "${output}" >&3
    fi
    [[ "${output}" == *"furyctl.yaml contains validation errors"* ]]
}

@test "valid furyctl yaml" {
    info
    test_dir="./automated-tests/integration/validation-cmd/test-data/valid-furyctl-yaml"
    abs_test_dir=${PWD}/${test_dir}
    furyctl_bin=${PWD}/dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl
    init(){
        cd ${test_dir} && ${furyctl_bin} -d --debug validate config --config ${abs_test_dir}/furyctl.yaml --distro-location ${abs_test_dir}
    }
    run init

    [ "${status}" -eq 0 ]
}
