# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

resource "aws_route53_zone" "zone" {
  name = "${var.cluster_name}.internal"

  vpc {
    vpc_id = module.vpc.vpc_id
  }
}

resource "aws_route53_record" "master" {
  count = length(aws_instance.master)

  zone_id = aws_route53_zone.zone.zone_id
  name    = "master${count.index + 1}.${var.cluster_name}.internal"
  type    = "A"
  ttl     = 300
  records = [aws_instance.master[count.index].private_ip]
}

resource "aws_route53_record" "worker" {
  count = length(aws_instance.worker)

  zone_id = aws_route53_zone.zone.zone_id
  name    = "worker${count.index + 1}.${var.cluster_name}.internal"
  type    = "A"
  ttl     = 300
  records = [aws_instance.worker[count.index].private_ip]
}
