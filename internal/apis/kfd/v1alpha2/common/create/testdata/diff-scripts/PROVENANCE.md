# Provenance

The `*.diff.sh.txt` files are rendered `scripts/diff.sh` files of the distribution, copied unchanged from the
template test baselines of `sighupio/distribution` (branch `feat/diff-script`, commit `5c3bc714`, 2026-10-21):

| File | Source in the distribution |
|---|---|
| `onpremises-02-full-cilium-haproxy.diff.sh.txt` | `tests/templates/onpremises/02-full-cilium-haproxy/baseline/scripts/diff.sh` |
| `kfddistribution-01-full-calico-nginx.diff.sh.txt` | `tests/templates/kfddistribution/01-full-calico-nginx/baseline/scripts/diff.sh` |

They use the `.txt` extension so that they are not taken for scripts of this repository. The tool paths are empty in
the baselines, as the distribution renders them in its tests.
