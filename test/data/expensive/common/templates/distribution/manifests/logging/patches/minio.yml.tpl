# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: minio-logging
  namespace: logging
spec:
  volumeClaimTemplates:
    - metadata:
        name: export-0
      spec:
        accessModes: [ "ReadWriteOnce" ]
        resources:
          requests:
            storage: {{ .spec.distribution.modules.logging.minio.storageSize }}
    - metadata:
        name: export-1
      spec:
        accessModes: [ "ReadWriteOnce" ]
        resources:
          requests:
            storage: {{ .spec.distribution.modules.logging.minio.storageSize }}