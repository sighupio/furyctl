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
# if [ "$(uname -m)" = "arm64" ]; then
# 	CPUARCH="arm64"
# fi

FURYCTL="${PWD}/dist/furyctl"

@test "prepare" {
    info
    init(){
        go build -o ${PWD}/dist/furyctl ${PWD}/main.go
    }
    run init
}

@test "furyctl" {
    info
    init(){
        ${FURYCTL} version
    }
    run init
    if [[ ${status} -ne 0 ]]; then
        echo "${output}" >&3
    fi
    [ "${status}" -eq 0 ]
}

@test "invalid furyctl yaml" {
    info
    test_dir="./test/integration/validation-cmd/data/config-invalid-furyctl-yaml"
    abs_test_dir=${PWD}/${test_dir}
    init(){
        cd ${test_dir} && ${FURYCTL} -d --debug validate config --config ${abs_test_dir}/furyctl.yaml --distro-location ${abs_test_dir}
    }
    run init

    [ "${status}" -eq 1 ]

    if [[ ${output} != *"validation failed"* ]]; then
        echo "${output}" >&3
    fi
    [[ "${output}" == *"validation failed"* ]]
}

@test "valid furyctl yaml" {
    info
    test_dir="./test/integration/validation-cmd/data/config-valid-furyctl-yaml"
    abs_test_dir=${PWD}/${test_dir}
    init(){
        cd ${test_dir} && ${FURYCTL} -d --debug validate config --config ${abs_test_dir}/furyctl.yaml --distro-location ${abs_test_dir}
    }
    run init

    [ "${status}" -eq 0 ]
}

@test "dependencies missing" {
    info
    test_dir="./test/integration/validation-cmd/data/dependencies-missing"
    abs_test_dir=${PWD}/${test_dir}
    init(){
        cd ${test_dir} && \
        ${FURYCTL} -d --debug \
            validate dependencies \
                --config ${abs_test_dir}/furyctl.yaml \
                --distro-location ${abs_test_dir} \
                --bin-path=${abs_test_dir}
    }
    run init

    [ "${status}" -eq 1 ]

    if [[ ${output} != *"ansible: no such file or directory"* ]] || \
        [[ ${output} != *"terraform: no such file or directory"* ]] || \
        [[ ${output} != *"kubectl: no such file or directory"* ]] || \
        [[ ${output} != *"kustomize: no such file or directory"* ]] || \
        [[ ${output} != *"furyagent: no such file or directory"* ]]; then
        echo "${output}" >&3
    fi

    [[ "${output}" == *"dependencies are not satisfied"* ]]
}

@test "wrong dependencies installed" {
    info
    test_dir="./test/integration/validation-cmd/data/dependencies-wrong"
    abs_test_dir=${PWD}/${test_dir}
    init(){
        cd ${test_dir} && \
        ${FURYCTL} -d --debug \
            validate dependencies \
                --config ${abs_test_dir}/furyctl.yaml \
                --distro-location ${abs_test_dir} \
                --bin-path=${abs_test_dir}
    }
    run init

    [ "${status}" -eq 1 ]

    if [[ ${output} != *"ansible version on system"* ]] || \
       [[ ${output} != *"terraform version on system"* ]] || \
       [[ ${output} != *"kubectl version on system"* ]] || \
       [[ ${output} != *"kustomize version on system"* ]] || \
       [[ ${output} != *"furyagent version on system"* ]]; then
        echo "${output}" >&3
    fi

    [[ "${output}" == *"dependencies are not satisfied"* ]]
}

@test "correct dependencies installed" {
    info
    test_dir="./test/integration/validation-cmd/data/dependencies-correct"
    abs_test_dir=${PWD}/${test_dir}
    init(){
        export AWS_ACCESS_KEY_ID=foo
        export AWS_SECRET_ACCESS_KEY=bar
        export AWS_DEFAULT_REGION=baz

        cd ${test_dir} && \
        ${FURYCTL} -d --debug \
            validate dependencies \
                --config ${abs_test_dir}/furyctl.yaml \
                --distro-location ${abs_test_dir} \
                --bin-path=${abs_test_dir}
    }
    run init

    [ "${status}" -eq 0 ]

    if [[ ${output} != *"ansible version on system"* ]] && \
       [[ ${output} != *"terraform version on system"* ]] && \
       [[ ${output} != *"kubectl version on system"* ]] && \
       [[ ${output} != *"kustomize version on system"* ]] && \
       [[ ${output} != *"furyagent version on system"* ]]; then
        echo "${output}" >&3
    fi

    [[ "${output}" == *"Dependencies validation succeeded"* ]]
}
