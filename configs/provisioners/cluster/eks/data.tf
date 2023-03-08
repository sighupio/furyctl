/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

data "aws_eks_cluster" "fury" {
  name = var.cluster_name
  depends_on = [
    module.fury
  ]
}

data "aws_eks_cluster_auth" "fury" {
  name = var.cluster_name
  depends_on = [
    module.fury
  ]
}
