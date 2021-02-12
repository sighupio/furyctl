#!/usr/bin/env bats
# Copyright (c) 2021 SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.


load "./../../helper"

OS="linux"
if [[ "$OSTYPE" == "darwin"* ]]; then
    OS="darwin"
fi

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

@test "Prepare bootstrap.yml file" {
    info
    init(){
        envsubst < ./automated-tests/e2e-tests/gcp-gke/bootstrap.tpl.yml > ./automated-tests/e2e-tests/gcp-gke/bootstrap.yml
    }
    run init
    [ "$status" -eq 0 ]
}

@test "Bootstrap init" {
    info
    init(){
        ./dist/furyctl-${OS}_${OS}_amd64/furyctl -d --debug bootstrap init --config ./automated-tests/e2e-tests/gcp-gke/bootstrap.yml -w ./automated-tests/e2e-tests/gcp-gke/bootstrap --reset
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
        ./dist/furyctl-${OS}_${OS}_amd64/furyctl -d --debug bootstrap apply --dry-run --config ./automated-tests/e2e-tests/gcp-gke/bootstrap.yml -w ./automated-tests/e2e-tests/gcp-gke/bootstrap
    }
    run apply

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "  TERRAFORM LOGS:" >&3
        cat ./automated-tests/e2e-tests/gcp-gke/bootstrap/logs/terraform.logs >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Bootstrap apply" {
    info
    apply(){
        ./dist/furyctl-${OS}_${OS}_amd64/furyctl -d --debug bootstrap apply --config ./automated-tests/e2e-tests/gcp-gke/bootstrap.yml -w ./automated-tests/e2e-tests/gcp-gke/bootstrap
    }
    run apply

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "  TERRAFORM LOGS:" >&3
        cat ./automated-tests/e2e-tests/gcp-gke/bootstrap/logs/terraform.logs >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Create openvpn profile" {
    info
    apply(){
        furyagent configure openvpn-client --client-name e2e-${CI_BUILD_NUMBER} --config ./automated-tests/e2e-tests/gcp-gke/bootstrap/secrets/furyagent.yml > /tmp/e2e.ovpn
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
        instance_ip=$(cat ./automated-tests/e2e-tests/gcp-gke/bootstrap/output/output.json | jq -r .vpn_ip.value[0])
        echo "  VPN Public IP: $instance_ip" >&3
        wait-for -t 60 $instance_ip:22 -- echo "VPN Instance $instance_ip SSH Port (22) UP!"
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
        tuns=$(netstat -i | grep tun0 | wc -l)
        if [ $tuns -eq 0 ]; then echo "VPN Connection not ready yet"; return 1; fi
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
        public_cidr=$(cat ./automated-tests/e2e-tests/gcp-gke/bootstrap/output/output.json | jq -r .public_subnets_cidr_blocks.value[0])
        echo "  Public CIDR: $public_cidr" >&3
        ips=$(nmap $public_cidr | grep -oE "\b([0-9]{1,3}\.){3}[0-9]{1,3}\b")
        for ip in $(echo $ips); do
            echo "  Public (internal) ip discovered: $ip" >&3
            timeout 3 ping -c1 $ip
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
        envsubst < ./automated-tests/e2e-tests/gcp-gke/cluster.tpl.yml > ./automated-tests/e2e-tests/gcp-gke/cluster.yml
    }
    run init
    [ "$status" -eq 0 ]
}

@test "Cluster init" {
    info
    init(){
        ./dist/furyctl-${OS}_${OS}_amd64/furyctl -d --debug cluster init --config ./automated-tests/e2e-tests/gcp-gke/cluster.yml -w ./automated-tests/e2e-tests/gcp-gke/cluster --reset
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
        ./dist/furyctl-${OS}_${OS}_amd64/furyctl -d --debug cluster apply --dry-run --config ./automated-tests/e2e-tests/gcp-gke/cluster.yml -w ./automated-tests/e2e-tests/gcp-gke/cluster
    }
    run apply

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "  TERRAFORM LOGS:" >&3
        cat ./automated-tests/e2e-tests/gcp-gke/cluster/logs/terraform.logs >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Cluster apply" {
    info
    apply(){
        ./dist/furyctl-${OS}_${OS}_amd64/furyctl -d --debug cluster apply --config ./automated-tests/e2e-tests/gcp-gke/cluster.yml -w ./automated-tests/e2e-tests/gcp-gke/cluster
    }
    run apply

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "  TERRAFORM LOGS:" >&3
        cat ./automated-tests/e2e-tests/gcp-gke/cluster/logs/terraform.logs >&3
    fi
    [ "$status" -eq 0 ]
}

@test "kubectl" {
    info
    cluster_info(){
        export KUBECONFIG=./automated-tests/e2e-tests/gcp-gke/cluster/secrets/kubeconfig
        kubectl get pods -A >&3
        kubectl get nodes -o wide >&3
    }
    run cluster_info

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        cat ./automated-tests/e2e-tests/gcp-gke/cluster/secrets/kubeconfig >&3
    fi
    [ "$status" -eq 0 ]
}

@test "Cluster destroy" {
    info
    destroy(){
        ./dist/furyctl-${OS}_${OS}_amd64/furyctl -d --debug cluster destroy --force --config ./automated-tests/e2e-tests/gcp-gke/cluster.yml -w ./automated-tests/e2e-tests/gcp-gke/cluster
    }
    run destroy

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "  TERRAFORM LOGS:" >&3
        cat ./automated-tests/e2e-tests/gcp-gke/cluster/logs/terraform.logs >&3
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
        ./dist/furyctl-${OS}_${OS}_amd64/furyctl -d --debug bootstrap destroy --force --config ./automated-tests/e2e-tests/gcp-gke/bootstrap.yml -w ./automated-tests/e2e-tests/gcp-gke/bootstrap
    }
    run destroy

    if [[ $status -ne 0 ]]; then
        echo "$output" >&3
        echo "  TERRAFORM LOGS:" >&3
        cat ./automated-tests/e2e-tests/gcp-gke/bootstrap/logs/terraform.logs >&3
    fi
    [ "$status" -eq 0 ]
}
