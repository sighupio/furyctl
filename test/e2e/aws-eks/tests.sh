#!/usr/bin/env bats
# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

load "./../../helper"

OS="linux"
if [[ "$OSTYPE" == "darwin"* ]]; then
    OS="darwin"
fi
CPUARCH="amd64_v1"
# if [ "$(uname -m)" = "arm64" ]; then
# 	CPUARCH="arm64"
# fi

@test "furyctl" {
    info
    init(){
        ./dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl --no-tty version
    }
    run init
    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Prepare bootstrap.yml file" {
    info
    init(){
        envsubst < ./test/e2e/aws-eks/bootstrap.tpl.yml > ./test/e2e/aws-eks/bootstrap.yml
    }
    run init
    [ "$status" -eq 0 ]
}

@test "Bootstrap init" {
    info
    init(){
        ./dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl --no-tty -d --debug bootstrap init --config ./test/e2e/aws-eks/bootstrap.yml -w ./test/e2e/aws-eks/bootstrap --reset
    }
    run init

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Bootstrap apply (dry-run)" {
    info
    apply(){
        ./dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl --no-tty -d --debug bootstrap apply --dry-run --config ./test/e2e/aws-eks/bootstrap.yml -w ./test/e2e/aws-eks/bootstrap
    }
    run apply

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "  TERRAFORM LOGS:" >&3
        cat ./test/e2e/aws-eks/bootstrap/logs/terraform.logs >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Bootstrap apply" {
    info
    apply(){
        ./dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl --no-tty -d --debug bootstrap apply --config ./test/e2e/aws-eks/bootstrap.yml -w ./test/e2e/aws-eks/bootstrap
    }
    run apply

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "  TERRAFORM LOGS:" >&3
        cat ./test/e2e/aws-eks/bootstrap/logs/terraform.logs >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Create openvpn profile" {
    info
    apply(){
        furyagent configure openvpn-client --client-name "e2e-${CI_BUILD_NUMBER}" --config ./test/e2e/aws-eks/bootstrap/secrets/furyagent.yml > /tmp/e2e.ovpn
    }
    run apply

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Wait for openvpn instance SSH port open" {
    info
    check(){
        instance_ip=$(jq -r .vpn_ip.value[0] ./test/e2e/aws-eks/bootstrap/output/output.json)
        echo "  VPN Public IP: $instance_ip" >&3
        wait-for -t 60 "$instance_ip:22" -- echo "VPN Instance $instance_ip SSH Port (22) UP!"
    }
    run check

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Connect to the vpn" {
    info
    apply(){
        vpn-connect /tmp/e2e.ovpn
    }
    vpntest(){
        tuns=$(netstat -i | grep -c tun0)
        if [ "$tuns" -eq 0 ]; then echo "VPN Connection not ready yet"; return 1; fi
    }
    run apply
    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "OVPN Profile: " >&3
        cat /tmp/e2e.ovpn >&3
    fi
    [ "$status" -eq 0 ]
    loop_it vpntest 60 5
    [ "$status" -eq 0 ]
}

@test "Test Ping" {
    info
    check(){
        public_cidr=$(jq -r .public_subnets_cidr_blocks.value[0] ./test/e2e/aws-eks/bootstrap/output/output.json)
        echo "  Public CIDR: $public_cidr" >&3
        ips=$(nmap "$public_cidr" | grep -oE "\b([0-9]{1,3}\.){3}[0-9]{1,3}\b")
        for ip in $ips; do
            echo "  Public (internal) ip discovered: $ip" >&3
            timeout 3 ping -c1 "$ip"
        done
    }
    run check

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Prepare cluster.yml file" {
    info
    init(){
        PRIVATE_SUBNETS=$(jq -r  .private_subnets.value ./test/e2e/aws-eks/bootstrap/output/output.json | tr -d '\n')
        export PRIVATE_SUBNETS
        NETWORK_ID=$(jq -r  .vpc_id.value ./test/e2e/aws-eks/bootstrap/output/output.json)
        export NETWORK_ID
        NETWORK_CIDR=$(jq -r  .vpc_cidr_block.value ./test/e2e/aws-eks/bootstrap/output/output.json)
        export NETWORK_CIDR
        envsubst < ./test/e2e/aws-eks/cluster.tpl.yml > ./test/e2e/aws-eks/cluster.yml
    }
    run init
    [ "$status" -eq 0 ]
}

@test "Cluster init" {
    info
    init(){
        ./dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl --no-tty -d --debug cluster init --config ./test/e2e/aws-eks/cluster.yml -w ./test/e2e/aws-eks/cluster --reset
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
        ./dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl --no-tty -d --debug cluster apply --dry-run --config ./test/e2e/aws-eks/cluster.yml -w ./test/e2e/aws-eks/cluster
    }
    run apply

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "  TERRAFORM LOGS:" >&3
        cat ./test/e2e/aws-eks/cluster/logs/terraform.logs >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Cluster apply" {
    info
    apply(){
        ./dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl --no-tty -d --debug cluster apply --config ./test/e2e/aws-eks/cluster.yml -w ./test/e2e/aws-eks/cluster
    }
    run apply

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "  TERRAFORM LOGS:" >&3
        cat ./test/e2e/aws-eks/cluster/logs/terraform.logs >&3
    fi
    [ "$status" -eq 0 ]
}

@test "kubectl get pods" {
    info
    cluster_info(){
        export KUBECONFIG=./test/e2e/aws-eks/cluster/secrets/kubeconfig
        kubectl get pods -A >&3
    }
    run cluster_info

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        cat ./test/e2e/aws-eks/cluster/secrets/kubeconfig >&3
    fi
    [ "$status" -eq 0 ]
}

@test "kubectl get nodes" {
    info
    cluster_info(){
        export KUBECONFIG=./test/e2e/aws-eks/cluster/secrets/kubeconfig
        kubectl get nodes -o wide --show-labels >&3
    }
    run cluster_info

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        cat ./test/e2e/aws-eks/cluster/secrets/kubeconfig >&3
    fi
    [ "$status" -eq 0 ]
}

@test "kubectl get nodes verify spot presence" {
    info
    test(){
        export KUBECONFIG=./test/e2e/aws-eks/cluster/secrets/kubeconfig
        data=$(kubectl get nodes --show-labels | grep "node.kubernetes.io/lifecycle=spot")
        if [ "${data}" == "" ]; then return 1; fi
    }
    loop_it test 60 5
    status=${loop_it_result}
    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        cat ./test/e2e/aws-eks/cluster/secrets/kubeconfig >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Cluster destroy" {
    info
    destroy(){
        ./dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl --no-tty -d --debug cluster destroy --force --config ./test/e2e/aws-eks/cluster.yml -w ./test/e2e/aws-eks/cluster
    }
    run destroy

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "  TERRAFORM LOGS:" >&3
        cat ./test/e2e/aws-eks/cluster/logs/terraform.logs >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Disconnect from the vpn" {
    info
    apply(){
        vpn-disconnect
    }
    run apply

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Bootstrap destroy" {
    info
    destroy(){
        ./dist/furyctl-${OS}_${OS}_${CPUARCH}/furyctl --no-tty -d --debug bootstrap destroy --force --config ./test/e2e/aws-eks/bootstrap.yml -w ./test/e2e/aws-eks/bootstrap
    }
    run destroy

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "  TERRAFORM LOGS:" >&3
        cat ./test/e2e/aws-eks/bootstrap/logs/terraform.logs >&3
    fi
    [ "$status" -eq 0 ]
}
