# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

resource "tls_private_key" "ssh_key" {
  algorithm = "ED25519"
}

resource "local_file" "ssh_private_key" {
  content  = tls_private_key.ssh_key.private_key_openssh
  filename = "${path.module}/secrets/ssh-private-key.pem"
}

resource "local_file" "ssh_public_key" {
  content  = tls_private_key.ssh_key.public_key_openssh
  filename = "${path.module}/secrets/ssh-public-key.pem"
}

resource "aws_key_pair" "key_pair" {
  key_name   = var.cluster_name
  public_key = tls_private_key.ssh_key.public_key_openssh
}
