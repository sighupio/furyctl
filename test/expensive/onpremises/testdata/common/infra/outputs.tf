# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

output "worker_private_ips" {
  value = aws_instance.worker[*].private_ip
}

output "master_private_ips" {
  value = aws_instance.master[*].private_ip
}
