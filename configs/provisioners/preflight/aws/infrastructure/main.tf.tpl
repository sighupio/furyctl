/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

terraform {
  backend "s3" {
    bucket = "{{ .terraform.backend.s3.bucketName }}"
    key    = "{{ .terraform.backend.s3.keyPrefix }}/infrastructure.json"
    region = "{{ .terraform.backend.s3.region }}"

    {{- if index .terraform.backend.s3 "skipRegionValidation" }}
      skip_region_validation = {{ default false .terraform.backend.s3.skipRegionValidation }}
    {{- end }}
  }
}
