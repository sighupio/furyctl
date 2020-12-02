locals {
  vpntemplate_vars = {
    openvpn_port           = var.vpn_port,
    openvpn_subnet_network = cidrhost(var.vpn_subnetwork_cidr, 0),
    openvpn_subnet_netmask = cidrnetmask(var.vpn_subnetwork_cidr),
    openvpn_routes         = [{ "network" : cidrhost(var.network_cidr, 0), "netmask" : cidrnetmask(var.network_cidr) }],
    openvpn_dns_servers    = [cidrhost(var.network_cidr, 2)], # The second ip is the DNS in AWS
    openvpn_dhparam_bits   = var.vpn_dhparams_bits,
    furyagent_version      = "v0.2.1"
    furyagent              = indent(6, local_file.furyagent.content),
  }

  furyagent_vars = {
    bucketName     = aws_s3_bucket.furyagent.bucket,
    aws_access_key = aws_iam_access_key.furyagent.id,
    aws_secret_key = aws_iam_access_key.furyagent.secret,
    region         = data.aws_region.current.name,
    servers        = [aws_eip.vpn.public_ip],
    user           = var.vpn_operator_name,
  }
  furyagent = templatefile("${path.module}/templates/furyagent.yml", local.furyagent_vars)
  users     = var.vpn_ssh_users
  sshkeys_vars = {
    users = local.users
  }
  sshkeys = templatefile("${path.module}/templates/ssh-users.yml", local.sshkeys_vars)
}

//INSTANCE RELATED STUFF

resource "aws_security_group" "vpn" {
  vpc_id      = module.vpc.vpc_id
  name_prefix = "${var.name}-"
}

resource "aws_security_group_rule" "vpn" {
  type              = "ingress"
  from_port         = var.vpn_port
  to_port           = var.vpn_port
  protocol          = "udp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.vpn.id
}

resource "aws_security_group_rule" "vpn_ssh" {
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = var.vpn_operator_cidrs
  security_group_id = aws_security_group.vpn.id
}

resource "aws_security_group_rule" "vpn_egress" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.vpn.id
}

resource "aws_eip" "vpn" {
  vpc = true
}

resource "aws_instance" "vpn" {
  ami                    = lookup(local.ubuntu_amis, data.aws_region.current.name, "")
  user_data              = templatefile("${path.module}/templates/vpn.yml", local.vpntemplate_vars)
  instance_type          = var.vpn_instance_type
  subnet_id              = module.vpc.public_subnets[0]
  vpc_security_group_ids = [aws_security_group.vpn.id]
  source_dest_check      = false
  root_block_device {
    volume_size = var.vpn_instance_disk_size
  }
}

resource "aws_eip_association" "vpn" {
  instance_id   = aws_instance.vpn.id
  allocation_id = aws_eip.vpn.id
}


// BUCKET AND IAM
resource "aws_s3_bucket" "furyagent" {
  bucket_prefix = "${var.name}-bootstrap-bucket-"
  acl           = "private"

  force_destroy = "true"

  versioning {
    enabled = true
  }

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "AES256"
      }
    }
  }
}

resource "aws_iam_user" "furyagent" {
  name = "${var.name}-bootstrap"
  path = "/"
}

resource "aws_iam_access_key" "furyagent" {
  user = aws_iam_user.furyagent.name
}

resource "aws_iam_policy_attachment" "furyagent" {
  name       = "${var.name}-bootstrap"
  users      = [aws_iam_user.furyagent.name]
  policy_arn = aws_iam_policy.furyagent.arn
}

resource "aws_iam_policy" "furyagent" {
  name = "${var.name}-bootstrap"

  policy = <<EOF
{
     "Version": "2012-10-17",
     "Statement": [
         {
             "Effect": "Allow",
             "Action": [
                 "s3:*"
             ],
            "Resource": "${aws_s3_bucket.furyagent.arn}/*"
         },
         {
             "Effect": "Allow",
             "Action": [
                 "s3:ListBucket",
                 "s3:GetBucketLocation"
             ],
            "Resource": "${aws_s3_bucket.furyagent.arn}"
         }
     ]
}
EOF
}

//FURYAGENT

resource "local_file" "furyagent" {
  content  = local.furyagent
  filename = "${path.root}/furyagent.yml"
}

resource "local_file" "sshkeys" {
  content  = local.sshkeys
  filename = "${path.root}/ssh-users.yml"
}

resource "null_resource" "init" {
  triggers = {
    "init" : "just-once",
  }
  provisioner "local-exec" {
    command = "until `furyagent init openvpn --config ${local_file.furyagent.filename}`; do echo \"Retrying\"; sleep 30; done" # Required because of aws iam lag
  }
}

resource "null_resource" "ssh_users" {
  triggers = {
    "sync" : join(",", local.users)
  }
  provisioner "local-exec" {
    command = "until `furyagent init ssh-keys --config ${local_file.furyagent.filename}`; do echo \"Retrying\"; sleep 30; done" # Required because of aws iam lag
  }
}
