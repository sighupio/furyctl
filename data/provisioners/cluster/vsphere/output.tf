/**
 * Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

output "ansible_inventory" {
  value       = module.fury.ansible_inventory
  description = "The ansible inventory to be used as hosts.ini file"
  sensitive   = true
}

output "haproxy_config" {
  value       = module.fury.haproxy_config
  description = "The ansible inventory to be used as hosts.ini file"
  sensitive   = true
}
