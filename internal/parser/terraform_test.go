// Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser_test

import (
	"reflect"
	"testing"

	"github.com/sighupio/furyctl/internal/parser"
)

func TestNewTfPlanParser(t *testing.T) {
	t.Parallel()

	type args struct {
		plan string
	}

	tests := []struct {
		name string
		args args
		want *parser.TfPlanParser
	}{
		{
			name: "test empty plan",
			args: args{
				plan: ``,
			},
			want: &parser.TfPlanParser{
				Plan: ``,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := parser.NewTfPlanParser(tt.args.plan); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewTfPlanParser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTfPlanParser_Parse(t *testing.T) {
	t.Parallel()

	type fields struct {
		plan string
	}

	tests := []struct {
		name   string
		fields fields
		want   *parser.TfPlan
	}{
		{
			name: "test empty plan",
			fields: fields{
				plan: ``,
			},
			want: &parser.TfPlan{
				Destroy: []string{},
				Add:     []string{},
				Change:  []string{},
			},
		},
		{
			name: "test plan with no changes",
			fields: fields{
				plan: `No changes. Infrastructure is up-to-date.`,
			},
			want: &parser.TfPlan{
				Destroy: []string{},
				Add:     []string{},
				Change:  []string{},
			},
		},
		{
			name: "test plan with add",
			fields: fields{
				plan: `Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

  # module.vpc[0].module.vpc.aws_eip.nat[0] will be created
  + resource "aws_eip" "nat" {
      + allocation_id        = (known after apply)
      + association_id       = (known after apply)
      + carrier_ip           = (known after apply)
      + customer_owned_ip    = (known after apply)
      + domain               = (known after apply)
      + id                   = (known after apply)
      + instance             = (known after apply)
      + network_border_group = (known after apply)
      + network_interface    = (known after apply)
      + private_dns          = (known after apply)
      + private_ip           = (known after apply)
      + public_dns           = (known after apply)
      + public_ip            = (known after apply)
      + public_ipv4_pool     = (known after apply)
      + tags                 = {
          + "Name"                                    = "furyctl-dev-eu-west-2a"
          + "kubernetes.io/cluster/furyctl-dev" = "shared"
        }
      + tags_all             = {
          + "Name"                                    = "furyctl-dev-eu-west-2a"
          + "env"                                     = "demo"
          + "kubernetes.io/cluster/furyctl-dev" = "shared"
        }
      + vpc                  = true
    }
  
  # module.vpc[0].module.vpc.aws_vpc.this[0] will be created
  + resource "aws_vpc" "this" {
      + arn                              = (known after apply)
      + assign_generated_ipv6_cidr_block = false
      + cidr_block                       = "10.0.0.0/16"
      + default_network_acl_id           = (known after apply)
      + default_route_table_id           = (known after apply)
      + default_security_group_id        = (known after apply)
      + dhcp_options_id                  = (known after apply)
      + enable_classiclink               = (known after apply)
      + enable_classiclink_dns_support   = (known after apply)
      + enable_dns_hostnames             = true
      + enable_dns_support               = true
      + id                               = (known after apply)
      + instance_tenancy                 = "default"
      + ipv6_association_id              = (known after apply)
      + ipv6_cidr_block                  = (known after apply)
      + main_route_table_id              = (known after apply)
      + owner_id                         = (known after apply)
      + tags                             = {
          + "Name"                                    = "furyctl-dev"
          + "kubernetes.io/cluster/furyctl-dev" = "shared"
        }
      + tags_all                         = {
          + "Name"                                    = "furyctl-dev"
          + "env"                                     = "demo"
          + "kubernetes.io/cluster/furyctl-dev" = "shared"
        }
    }

Plan: 2 to add, 0 to change, 0 to destroy.

Changes to Outputs:
  + vpc_cidr_block              = "10.0.0.0/16"
  + vpc_id                      = (known after apply)`,
			},
			want: &parser.TfPlan{
				Destroy: []string{},
				Add:     []string{"aws_eip", "aws_vpc"},
				Change:  []string{},
			},
		},
		{
			name: "test plan with destroy/create",
			fields: fields{
				plan: `Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
-/+ destroy and then create replacement

Terraform will perform the following actions:

  # module.vpc[0].module.vpc.aws_route_table_association.private[2] must be replaced
-/+ resource "aws_route_table_association" "private" {
      ~ id             = "rtbassoc-076264f69a6f05f84" -> (known after apply)
      ~ subnet_id      = "subnet-0bf83e961b95609f1" -> (known after apply) # forces replacement
        # (1 unchanged attribute hidden)
    }

  # module.vpc[0].module.vpc.aws_subnet.private[2] must be replaced
-/+ resource "aws_subnet" "private" {
      ~ arn                             = "arn:aws:ec2:eu-west-2:123456789123:subnet/subnet-0bf83e961b95609f1" -> (known after apply)
      ~ availability_zone_id            = "euw2-az1" -> (known after apply)
      ~ cidr_block                      = "10.0.162.0/24" -> "10.0.152.0/24" # forces replacement
      ~ id                              = "subnet-0bf83e961b95609f1" -> (known after apply)
      + ipv6_cidr_block_association_id  = (known after apply)
      - map_customer_owned_ip_on_launch = false -> null
      ~ owner_id                        = "492816857163" -> (known after apply)
        tags                            = {
            "Name"                                    = "furyctl-dev-private-eu-west-2c"
            "kubernetes.io/cluster/furyctl-dev" = "shared"
            "kubernetes.io/role/internal-elb"         = "1"
        }
        # (5 unchanged attributes hidden)
    }

Plan: 2 to add, 0 to change, 2 to destroy.

Changes to Outputs:
  ~ private_subnets             = [
        # (1 unchanged element hidden)
        "subnet-03056835b05d70b7f",
      - "subnet-0bf83e961b95609f1",
      + (known after apply),
    ]
  ~ private_subnets_cidr_blocks = [
        # (1 unchanged element hidden)
        "10.0.172.0/24",
      - "10.0.162.0/24",
      + "10.0.152.0/24",
    ]`,
			},
			want: &parser.TfPlan{
				Destroy: []string{"aws_route_table_association", "aws_subnet"},
				Add:     []string{"aws_route_table_association", "aws_subnet"},
				Change:  []string{},
			},
		},
		{
			name: "test plan with update in-place",
			fields: fields{
				plan: `Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  ~ update in-place
-/+ destroy and then create replacement

Terraform will perform the following actions:

  # module.vpn[0].aws_eip.vpn[0] will be updated in-place
  ~ resource "aws_eip" "vpn" {
        id                   = "eipalloc-02035b7d0eaec0b1c"
        tags                 = {}
      ~ tags_all             = {
          + "env" = "demo"
        }
        # (11 unchanged attributes hidden)
    }

  # module.vpn[0].local_file.sshkeys must be replaced
-/+ resource "local_file" "sshkeys" {
      ~ content              = <<-EOT # forces replacement
            users:
              - name: Text-x
                user_id: Text-x
          +   - name: Test
          +     user_id: Test
        EOT
      ~ id                   = "c739e95cf668b706029aac0d0b428bb3fa16a94b" -> (known after apply)
        # (3 unchanged attributes hidden)
    }

  # module.vpn[0].null_resource.ssh_users must be replaced
-/+ resource "null_resource" "ssh_users" {
      ~ id       = "1885518357807060104" -> (known after apply)
      ~ triggers = { # forces replacement
          ~ "sync-users"    = "Text-x" -> "Text-x,Test"
            # (1 unchanged element hidden)
        }
    }

Plan: 2 to add, 1 to change, 2 to destroy.`,
			},
			want: &parser.TfPlan{
				Destroy: []string{"local_file", "null_resource"},
				Add:     []string{"local_file", "null_resource"},
				Change:  []string{"aws_eip"},
			},
		},
		{
			name: "test plan with destroy",
			fields: fields{
				plan: `Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  - destroy

Terraform will perform the following actions:

  # module.vpn[0].aws_eip.vpn[0] will be destroyed
  - resource "aws_eip" "vpn" {
      - association_id       = "eipassoc-03acc973673630e2c" -> null
      - domain               = "vpc" -> null
      - id                   = "eipalloc-02035b7d0eaec0b1c" -> null
      - instance             = "i-02e52a79b1fae934b" -> null
      - network_border_group = "eu-west-2" -> null
      - network_interface    = "eni-0f0f91076c9b3ca5b" -> null
      - private_dns          = "ip-10-0-20-7.eu-west-2.compute.internal" -> null
      - private_ip           = "10.0.20.7" -> null
      - public_dns           = "ec2-13-42-227-161.eu-west-2.compute.amazonaws.com" -> null
      - public_ip            = "13.42.227.161" -> null
      - public_ipv4_pool     = "amazon" -> null
      - tags                 = {} -> null
      - tags_all             = {} -> null
      - vpc                  = true -> null
    }

  # module.vpn[0].aws_eip_association.vpn[0] will be destroyed
  - resource "aws_eip_association" "vpn" {
      - allocation_id        = "eipalloc-02035b7d0eaec0b1c" -> null
      - id                   = "eipassoc-03acc973673630e2c" -> null
      - instance_id          = "i-02e52a79b1fae934b" -> null
      - network_interface_id = "eni-0f0f91076c9b3ca5b" -> null
      - private_ip_address   = "10.0.20.7" -> null
      - public_ip            = "13.42.227.161" -> null
    }

Plan: 0 to add, 0 to change, 2 to destroy.

Changes to Outputs:
  - furyagent = (sensitive value)
  - vpn_ip    = [
      - "13.42.227.161",
    ] -> null
`,
			},
			want: &parser.TfPlan{
				Destroy: []string{"aws_eip", "aws_eip_association"},
				Add:     []string{},
				Change:  []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &parser.TfPlanParser{
				Plan: tt.fields.plan,
			}

			got := p.Parse()

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse() got = %v, want %v", got, tt.want)
			}
		})
	}
}
