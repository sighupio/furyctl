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

FURYCTL="${PWD}/dist/furyctl_${OS}_${CPUARCH}/furyctl"

@test "furyctl" {
    info
    init(){
        "${FURYCTL}" version
    }
    run init
    if [[ ${status} -ne 0 ]]; then
        echo "${output}" >&3
    fi
    [ "${status}" -eq 0 ]
}

@test "no distribution file" {
    info
    test_dir="./test/integration/template-engine/data/no-distribution-yaml"
    init(){
        cd ${test_dir} && ${FURYCTL} -d --debug dump template
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
    test_dir="./test/integration/template-engine/data/no-furyctl-yaml"
    init(){
        cd ${test_dir} && ${FURYCTL} -d --debug dump template
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
    test_dir="./test/integration/template-engine/data/distribution-yaml-no-data-property"
    init(){
        cd ${test_dir} && ${FURYCTL} -d --debug dump template
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
    test_dir="./test/integration/template-engine/data/empty"
    init(){
        cd ${test_dir} && ${FURYCTL} -d --debug dump template
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
    test_dir="./test/integration/template-engine/data/simple-dry-run"
    init(){
        cd ${test_dir} && ${FURYCTL} -d --debug dump template --dry-run
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
    test_dir="./test/integration/template-engine/data/simple"
    init(){
        cd ${test_dir} && ${FURYCTL} -d --debug dump template
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
    test_dir="./test/integration/template-engine/data/complex"
    init(){
        cd ${test_dir} && ${FURYCTL} -d --debug dump template --dry-run

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
    test_dir="./test/integration/template-engine/data/complex"
    init(){
        cd ${test_dir} && ${FURYCTL} -d --debug dump template

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
