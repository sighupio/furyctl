#!/usr/bin/env bats
# Copyright (c) 2021 SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.


load "./../../helper"

OS="linux"
if [[ "$OSTYPE" == "darwin"* ]]; then
    OS="darwin"
fi
CPUARCH="amd64"
if [[ $(uname -m) -eq "arm64" ]]; then
	CPUARCH="arm64"
fi

@test "furyctl" {
    info
    init(){
        ./dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl version
    }
    run init
    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Bootstrap init" {
    info
    init(){
        ./dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl -d --debug bootstrap init --config ./automated-tests/integration/aws-eks/bootstrap.yml -w ./automated-tests/integration/aws-eks/bootstrap --reset
    }
    run init

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Bootstrap structure" {
    info
    project_dir="./automated-tests/integration/aws-eks/bootstrap"
    test(){
        if [ -e ${project_dir}/bin/terraform ] && [ -e ${project_dir}/configuration/.netrc ] && [ -e ${project_dir}/logs/terraform.logs ] && [ -e ${project_dir}/.gitignore ] && [ -e ${project_dir}/.gitattributes ] && [ -e ${project_dir}/.gitattributes ]
        then
            echo "  All files exist, directory intact" >&3
            return 0
        else
            echo "  One or more files are missing" >&3
            return 1
        fi
    }
    run test
    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Cluster init" {
    info
    init(){
        ./dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl -d --debug cluster init --config ./automated-tests/integration/aws-eks/cluster.yml -w ./automated-tests/integration/aws-eks/cluster --reset
    }
    run init
    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Cluster structure" {
    info
    project_dir="./automated-tests/integration/aws-eks/cluster"
    test(){
        if [ -e ${project_dir}/bin/terraform ] && [ -e ${project_dir}/configuration/.netrc ] && [ -e ${project_dir}/logs/terraform.logs ] && [ -e ${project_dir}/.gitignore ] && [ -e ${project_dir}/.gitattributes ] && [ -e ${project_dir}/backend.tf ]
        then
            echo "  All files exist, directory intact" >&3
            return 0
        else
            echo "  One or more files are missing" >&3
            return 1
        fi
    }
    run test
    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
    fi
    [ "$status" -eq 0 ]
}
