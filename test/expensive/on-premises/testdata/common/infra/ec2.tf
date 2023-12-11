# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

data "aws_ami" "ubuntu" {
  most_recent = true

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-focal-20.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  owners = ["099720109477"]
}

resource "aws_security_group" "security_group" {
  name   = var.cluster_name
  vpc_id = module.vpc.vpc_id

  ingress {
    from_port        = 0
    to_port          = 0
    protocol         = "-1"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
  }

  egress {
    from_port        = 0
    to_port          = 0
    protocol         = "-1"
    cidr_blocks      = ["0.0.0.0/0"]
    ipv6_cidr_blocks = ["::/0"]
  }

  tags = {
    Name = var.cluster_name
  }
}

resource "aws_network_interface" "master" {
  count = 1

  subnet_id       = module.vpc.private_subnets[0]
  security_groups = [aws_security_group.security_group.id]

  tags = {
    Name = "${var.cluster_name}-master-${count.index + 1}"
  }
}

resource "aws_instance" "master" {
  count = 1

  ami           = data.aws_ami.ubuntu.id
  instance_type = "t3.medium"
  key_name      = aws_key_pair.key_pair.key_name

  network_interface {
    network_interface_id = aws_network_interface.master[count.index].id
    device_index         = 0
  }

  tags = {
    Name = "${var.cluster_name}-master-${count.index + 1}"
  }
}

resource "aws_network_interface" "worker" {
  count = 3

  subnet_id       = module.vpc.private_subnets[0]
  security_groups = [aws_security_group.security_group.id]

  tags = {
    Name = "${var.cluster_name}-worker-${count.index + 1}"
  }
}

resource "aws_instance" "worker" {
  count = 3

  ami           = data.aws_ami.ubuntu.id
  instance_type = "t3.large"
  key_name      = aws_key_pair.key_pair.key_name

  network_interface {
    network_interface_id = aws_network_interface.worker[count.index].id
    device_index         = 0
  }

  tags = {
    Name = "${var.cluster_name}-worker-${count.index + 1}"
  }
}
