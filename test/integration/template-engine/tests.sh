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

@test "no distribution file" {
    info
    test_dir="./test/integration/template-engine/test-data/no-distribution-yaml"
    furyctl_bin=${PWD}/dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl
    init(){
        cd ${test_dir} && ${furyctl_bin} -d --debug template
    }
    run init

    [ "${status}" -eq 1 ]

    if [[ ${output} != *"distribution.yaml: no such file or directory"* ]]; then
        echo "${output}" >&3
    fi
    [[ "${output}" == *"distribution.yaml: no such file or directory"* ]]
}

@test "no furyctl.yaml file" {
    info
    test_dir="./test/integration/template-engine/test-data/no-furyctl-yaml"
    furyctl_bin=${PWD}/dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl
    init(){
        cd ${test_dir} && ${furyctl_bin} -d --debug template
    }
    run init

    [ "${status}" -eq 1 ]

    if [[ ${output} != *"furyctl.yaml: no such file or directory"* ]]; then
        echo "${output}" >&3
    fi
    [[ "${output}" == *"furyctl.yaml: no such file or directory"* ]]
}

@test "no data property in distribution.yaml file" {
    info
    test_dir="./test/integration/template-engine/test-data/distribution-yaml-no-data-property"
    furyctl_bin=${PWD}/dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl
    init(){
        cd ${test_dir} && ${furyctl_bin} -d --debug template
    }
    run init

    [ "${status}" -eq 1 ]

    if [[ ${output} != *"incorrect base file, cannot access key data on map"* ]]; then
        echo "${output}" >&3
    fi
    [[ "${output}" == *"incorrect base file, cannot access key data on map"* ]]
}

@test "empty template" {
    info
    test_dir="./test/integration/template-engine/test-data/empty"
    furyctl_bin=${PWD}/dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl
    init(){
        cd ${test_dir} && ${furyctl_bin} -d --debug template
        if [ -f ./target/file.txt ]; then false; else true; fi
    }
    run init

    if [[ ${status} -ne 0 ]]; then
        echo "${output}" >&3
    fi
    [ "${status}" -eq 0 ]
}

@test "simple template dry-run" {
    info
    test_dir="./test/integration/template-engine/test-data/simple-dry-run"
    furyctl_bin=${PWD}/dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl
    init(){
        cd ${test_dir} && ${furyctl_bin} -d --debug template --dry-run
        cat ./target/file.txt | grep "testValue"
    }
    run init

    if [[ ${status} -ne 0 ]]; then
        echo "${output}" >&3
    fi
    [ "${status}" -eq 0 ]
}

@test "simple template" {
    info
    test_dir="./test/integration/template-engine/test-data/simple"
    furyctl_bin=${PWD}/dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl
    init(){
        cd ${test_dir} && ${furyctl_bin} -d --debug template
        cat ./target/file.txt | grep "testValue"
    }
    run init

    if [[ ${status} -ne 0 ]]; then
        echo "${output}" >&3
    fi
    [ "${status}" -eq 0 ]
}

@test "complex template dry-run" {
    info
    test_dir="./test/integration/template-engine/test-data/complex"
    furyctl_bin=${PWD}/dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl
    init(){
        cd ${test_dir} && ${furyctl_bin} -d --debug template --dry-run

        # test that the config/example.yaml file has been generated
        if [ ! -f ./target/config/example.yaml ]; then
            echo "config/example.yaml file not generated" >&3
            false
        fi

        # test that the kustomization.yaml file has been generated
        if [ ! -f ./target/kustomization.yaml ]; then
            echo "kustomization.yaml file not generated" >&3
            false
        fi

        # test that the config/example.yaml contains the string "configdata: example"
        if ! grep -q "configdata: example" ./target/config/example.yaml; then
            echo "config/example.yaml file does not contain the string 'configdata: example'" >&3
            false
        fi

        # test that the kustomization.yaml contains the same data of data/expected-kustomization.yaml
        if ! diff -q ./target/kustomization.yaml ./data/expected-kustomization.yaml; then
            echo -e "kustomization.yaml file does not contain the same data of data/expected-kustomization.yaml, Diff:" >&3
            diff ./target/kustomization.yaml ./data/expected-kustomization.yaml >&3
            false
        fi
    }
    run init

    if [[ ${status} -ne 0 ]]; then
        echo "${output}" >&3
    fi
    [ "${status}" -eq 0 ]
}

@test "complex template" {
    info
    test_dir="./test/integration/template-engine/test-data/complex"
    furyctl_bin=${PWD}/dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl
    init(){
        cd ${test_dir} && ${furyctl_bin} -d --debug template

        # test that the config/example.yaml file has been generated
        if [ ! -f ./target/config/example.yaml ]; then
            echo "config/example.yaml file not generated" >&3
            false
        fi

        # test that the kustomization.yaml file has been generated
        if [ ! -f ./target/kustomization.yaml ]; then
            echo "kustomization.yaml file not generated" >&3
            false
        fi

        # test that the config/example.yaml contains the string "configdata: example"
        if ! grep -q "configdata: example" ./target/config/example.yaml; then
            echo "config/example.yaml file does not contain the string 'configdata: example'" >&3
            false
        fi

        # test that the kustomization.yaml contains the same data of data/expected-kustomization.yaml
        if ! diff -q ./target/kustomization.yaml ./data/expected-kustomization.yaml; then
            echo -e "kustomization.yaml file does not contain the same data of data/expected-kustomization.yaml, Diff:" >&3
            diff ./target/kustomization.yaml ./data/expected-kustomization.yaml >&3
            false
        fi
    }
    run init

    if [[ ${status} -ne 0 ]]; then
        echo "${output}" >&3
    fi
    [ "${status}" -eq 0 ]
}
