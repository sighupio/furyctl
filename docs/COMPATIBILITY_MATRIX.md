# Furyctl and KFD compatibility

Note: Always use the latest `furyctl` version, we make sure that is compatible with all the last 3 minor KFD versions.

| Furyctl / KFD | 1.27.3             | 1.27.2             | 1.27.1             | 1.27.0             | 1.26.5             | 1.26.4             | 1.26.3             | 1.25.10            | 1.25.9             | 1.25.8             |
| ------------- | ------------------ | ------------------ | ------------------ | ------------------ | ------------------ | ------------------ | ------------------ | ------------------ | ------------------ | ------------------ |
| 0.27.3        | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |
| 0.27.2        |                    | :white_check_mark: | :white_check_mark: | :white_check_mark: |                    | :white_check_mark: | :white_check_mark: |                    | :white_check_mark: | :white_check_mark: |
| 0.27.1        |                    |                    | :white_check_mark: | :white_check_mark: |                    | :white_check_mark: | :white_check_mark: |                    | :white_check_mark: | :white_check_mark: |
| 0.27.0        |                    |                    | :white_check_mark: | :white_check_mark: |                    | :white_check_mark: | :white_check_mark: |                    | :white_check_mark: | :white_check_mark: |

## Furyctl and Providers compatibility

| Furyctl / Providers | EKSCluster         | KFDDistribution    | OnPremises         |
| ------------------- | ------------------ | ------------------ | ------------------ |
| 0.27.3              | :white_check_mark: | :white_check_mark: | :white_check_mark: |
| 0.27.2              | :white_check_mark: | :white_check_mark: | :white_check_mark: |
| 0.27.1              | :white_check_mark: | :white_check_mark: | :white_check_mark: |
| 0.27.0              | :white_check_mark: | :white_check_mark: | :white_check_mark: |
| 0.26.3              | :white_check_mark: | :white_check_mark: | :white_check_mark: |
| 0.26.2              | :white_check_mark: | :white_check_mark: | :white_check_mark: |
| 0.26.1              | :white_check_mark: | :white_check_mark: |                    |
| 0.26.0              | :white_check_mark: | :white_check_mark: |                    |
| 0.25.2              | :white_check_mark: | :white_check_mark: |                    |
| 0.25.1              | :white_check_mark: | :white_check_mark: |                    |
| 0.25.0              | :white_check_mark: | :white_check_mark: |                    |
| 0.25.0-beta.0       | :white_check_mark: |                    |                    |
| 0.25.0-alpha.1      | :white_check_mark: |                    |                    |

## Legacy compatibility

These versions were still not using the paradigm to have a full backward compatibility with the latest 3 minor versions of KFD.

| Furyctl / KFD  | 1.26.3             | 1.26.2             | 1.26.1             | 1.26.0             | 1.25.9             | 1.25.8             | 1.25.7             | 1.25.6             | 1.25.5             | 1.25.4             | 1.25.3             | 1.25.2             |
| -------------- | ------------------ | ------------------ | ------------------ | ------------------ | ------------------ | ------------------ | ------------------ | ------------------ | ------------------ | ------------------ | ------------------ | ------------------ |
| 0.26.3         | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |                    | :white_check_mark: |                    |                    |                    |                    |                    |                    |
| 0.26.2         | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |                    | :white_check_mark: |                    |                    |                    |                    |                    |                    |
| 0.26.1         | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |                    |                    |                    |                    |                    |                    |                    |                    |
| 0.26.0         | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |                    |                    |                    |                    |                    |                    |                    |                    |
| 0.25.2         | :warning:          | :warning:          | :warning:          | :warning:          |                    |                    | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |                    |                    |
| 0.25.1         |                    |                    |                    |                    |                    |                    | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |                    |                    |
| 0.25.0         |                    |                    |                    |                    |                    |                    | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: |                    |                    |
| 0.25.0-beta.0  |                    |                    |                    |                    |                    |                    |                    |                    |                    |                    | :white_check_mark: |                    |
| 0.25.0-alpha.1 |                    |                    |                    |                    |                    |                    |                    |                    |                    |                    |                    | :white_check_mark: |
