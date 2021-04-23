#!/usr/bin/env bats
# Copyright (c) 2021 SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.


load "./../../helper"

OS="linux"
if [[ "$OSTYPE" == "darwin"* ]]; then
    OS="darwin"
fi

@test "Prepare temporal ssh key" {
    info
    ssh_keys(){
        ssh-keygen -b 2048 -t rsa -f /tmp/sshkey -q -N ""
    }
    run ssh_keys
    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
    fi
    [ "$status" -eq 0 ]
}

@test "furyctl" {
    info
    init(){
        ./dist/furyctl-${OS}_${OS}_amd64/furyctl version
    }
    run init
    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Prepare cluster.yml file" {
    info
    init(){
        envsubst < ./automated-tests/e2e-tests/vsphere/cluster.tpl.yml > ./automated-tests/e2e-tests/vsphere/cluster.yml
    }
    run init
    [ "$status" -eq 0 ]
}

@test "Cluster init" {
    info
    init(){
        ./dist/furyctl-${OS}_${OS}_amd64/furyctl -d --debug cluster init --config ./automated-tests/e2e-tests/vsphere/cluster.yml -w ./automated-tests/e2e-tests/vsphere/cluster --reset
    }
    run init

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Cluster apply (dry-run)" {
    info
    apply(){
        ./dist/furyctl-${OS}_${OS}_amd64/furyctl -d --debug cluster apply --dry-run --config ./automated-tests/e2e-tests/vsphere/cluster.yml -w ./automated-tests/e2e-tests/vsphere/cluster
    }
    run apply

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "  TERRAFORM LOGS:" >&3
        cat ./automated-tests/e2e-tests/vsphere/cluster/logs/terraform.logs >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Cluster apply" {
    info
    apply(){
        ./dist/furyctl-${OS}_${OS}_amd64/furyctl -d --debug cluster apply --config ./automated-tests/e2e-tests/vsphere/cluster.yml -w ./automated-tests/e2e-tests/vsphere/cluster
    }
    run apply

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "  TERRAFORM LOGS:" >&3
        cat ./automated-tests/e2e-tests/vsphere/cluster/logs/terraform.logs >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Ping" {
    info
    ping(){
        cd ./automated-tests/e2e-tests/vsphere/cluster/provision && ansible all -m ping
    }
    run ping

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        cat ./automated-tests/e2e-tests/vsphere/cluster/secrets/kubeconfig >&3
    fi
    [ "$status" -eq 0 ]
}

@test "kubectl" {
    info
    cluster_info(){
        export KUBECONFIG=./automated-tests/e2e-tests/vsphere/cluster/secrets/kubeconfig
        kubectl get pods -A >&3
        kubectl get nodes -o wide >&3
    }
    run cluster_info

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        cat ./automated-tests/e2e-tests/vsphere/cluster/secrets/kubeconfig >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Cluster destroy" {
    info
    destroy(){
        ./dist/furyctl-${OS}_${OS}_amd64/furyctl -d --debug cluster destroy --force --config ./automated-tests/e2e-tests/vsphere/cluster.yml -w ./automated-tests/e2e-tests/vsphere/cluster
    }
    run destroy

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "  TERRAFORM LOGS:" >&3
        cat ./automated-tests/e2e-tests/vsphere/cluster/logs/terraform.logs >&3
    fi
    [ "$status" -eq 0 ]
}
