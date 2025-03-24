# Development FAQ

### **How to trace the workflow of various commands, for example, where can we find the full flow of the `apply` command for the OnPremises provider, and where is the definition of the various phases?**

<details>
<summary>Answer</summary>

Everything starts from `main.go`, which executes the root command `cmd/root.go` created with the `github.com/spf13/cobra` library. Cobra is a popular library for managing commands in Go, and it provides a clear and scalable structure for adding new functionalities or commands to the project.

The root command includes all other commands, for example, `cmd/apply.go`, which handles the application of configurations. Specifically, for the `OnPremises` provider, we can follow the logic defined in the `RunE` method found in `internal/cluster/creator.go:63`. Here, the "Creator" is initialized to manage and create clusters based on the `Kind` type. For `OnPremises`, the logic for creation is located in `internal/apis/kfd/v1alpha2/onpremises/creator.go`. Once the Creator is initialized, the `Create` function (at `internal/apis/kfd/v1alpha2/onpremises/creator.go:171`) is called to actually apply the configuration and start the process.

The four phases definition and handling depend only on the concrete implementation of the `ClusterCreator`, for example, in the `OnPremises` provider, we have it in `internal/apis/kfd/v1alpha2/onpremises/creator.go:276`. There are two switches that can tell `furyctl` from which phase to start or which phase to run (`--start-from` and `--phase`).

</details>

---

### **How does `furyctl` use the library generated in the `fury-distribution` repository?**

<details>
<summary>Answer</summary>

The library generated in `fury-distribution` is mainly used for parsing the `furyctl.yaml` file, as it provides the necessary data structures for representing configurations in Go. The variables in the `furyctl` code are mapped to these data structures to ensure that the data is interpreted correctly during the execution of commands.

The decision to separate the data structures between `fury-distribution` and `furyctl` stemmed from an initial design vision to have a versioned schema for the configuration, which would allow for better management of evolving structures over time. However, this approach was never fully implemented, and this division might be reviewed and potentially eliminated, centralizing the management of the configuration in one place.

</details>

---

### **How are tests executed in the project (both unit and e2e)?**

<details>
<summary>Answer</summary>

Unit tests in the project follow Go’s standard testing framework and are integrated into the codebase. These tests are designed to verify the functionality of individual components, ensuring that each part of the code works as expected in isolation. Unit tests can be executed easily by running the command `make test-unit`. This command triggers the Go test suite, which looks for test functions (those prefixed with `Test`) in the relevant packages and runs them.

End-to-End (E2E) tests, which are more comprehensive and typically involve interactions with external systems (like Kubernetes clusters), are also part of the project. These tests simulate real-world scenarios to ensure the entire system works as expected when all components are integrated. For example, E2E tests include creating and managing EKS (Elastic Kubernetes Service) clusters, verifying that the system behaves correctly in a real cluster environment. These tests can be triggered by running the `make test-expensive` command. The reason for this designation as "expensive" is that E2E tests often involve external dependencies, such as Kubernetes clusters, which can take time to set up and may incur additional costs if used in a cloud environment.

Expensive tests have historically been only run locally by hand, in CI `test-expensive` are never run.

There are also basic E2E tests (`make test-e2e`) that execute some `furyctl` commands to verify its functionality. The cluster creation commands are run with the `--dry-run` flag.

</details>

---

### **What does the directory structure look like and what is the purpose of each directory?**

<details>
<summary>Answer</summary>

The project structure is divided into several directories with specific responsibilities:

- `cmd`: Contains the main commands created with the Cobra library. Each command represents a distinct functionality of the CLI (e.g., `apply`, `delete`, etc.).
- `configs`: Contains the patch configurations and upgrade paths for the distribution.
  - `patches`: Contains the patches applied by replacing files in the `fury-distribution` repo previously downloaded, categorized by specific version.
  - `provisioners`: Contains some terraform templates that will be filled during the PreFlight phase in the EKS provider.
  - `upgrades`: Contains folders categorized by upgrade version, for example, `1.29.5-1.30.0` for an upgrade from `v1.29.5` to `v1.30.0`. These folders contain hooks such as `pre-distribution.sh.tpl`, which is executed before the `distribution` phase. The hooks follow the structure `{pre|post}-{phase}.sh.tpl`.
- `docs`: Contains the project documentation, including changelogs and other information.
- `internal`: Contains code that is not meant to be exported, related to `furyctl`. It includes private implementations that should not be used outside the package.
- `pkg`: Contains code that is exposed as APIs, meant to be used by other packages or projects.
- `test`: Contains the data used in tests, including test configurations and assets needed to run the tests.

In Go projects, the `pkg` and `internal` directories serve distinct purposes, based on Go's package visibility rules and the intended structure of the code. Here's how they differ:

- **`pkg`**: The `pkg` directory contains code that is **publicly available to other projects** or packages. This means that the code inside `pkg` is designed to be used by external consumers of your project. It includes libraries, utilities, or APIs that are intended to be shared, reused, or extended outside the project. For example, the `pkg` directory may contain core business logic, models, and utility functions that other projects can import and use.

- **`internal`**: The `internal` directory is for code that is **only meant to be used within the project**. Code inside this directory is **not accessible to external projects** or even to any packages outside of the current module. This provides a level of encapsulation, ensuring that only the code within the module itself can access and use the internal functionality. For instance, `internal` might contain helper functions, implementations, or data structures that are needed for the internal workings of the project but shouldn't be exposed as part of the public API.
</details>

---

### **Where in the code does the merge of defaults with the user-provided `furyctl.yaml` file happen, and how is the state managed in the cluster (secrets written to the `kube-system` namespace)?**

<details>
<summary>Answer</summary>

The merge of defaults happens primarily for the distribution configurations. When a user provides a `furyctl.yaml` file, the default values for the distribution are overwritten by the user-defined configurations, but only for the settings explicitly defined in the YAML file. This approach allows applying a custom configuration without losing the base configuration. The defaults are only for distribution configuration.

The cluster state is monitored by comparing the current configuration with the desired one. Specifically, when `furyctl` writes information, such as secrets `furyctl-config` and `furyctl-kfd` in the `kube-system` namespace, this is used to determine which changes have been made and what needs to be updated or created. This information is then used to synchronize the cluster state with the specified configuration. When we run the `apply` command, `furyctl` saves the current `furyctl.yaml` file inside a Kubernetes secret. For subsequent calls to `apply`, the secret is read and decoded, then we diff it against the current and compared. Depending on the differences, `furyctl` decides what to do (explained in fury-distribution docs).

</details>

---

### **What are the libraries included in `go.mod` and their purposes (how are they used in the code)?**

<details>
<summary>Answer</summary>

The libraries included in `go.mod` are relatively few, and their purpose is straightforward to understand by exploring the codebase. Here are some key libraries used in the project:

- **`github.com/spf13/cobra`**: This is the main library used for building the CLI commands. It helps structure the commands, arguments, flags, and the overall command-line interface. It’s used in the `cmd` directory to define commands such as `apply`, `delete`, and others.
- **`github.com/sirupsen/logrus`**: This is the logging library used to handle logging in the project. It's a popular choice for structured logging in Go. It is used throughout the codebase to log various events, including errors, info messages, and debug output.
- **`github.com/santhosh-tekuri/jsonschema`**: This library is used to validate JSON schema. It’s used to validate the structure of the `furyctl.yaml` file, ensuring that the user configuration adheres to the expected format. We use it to check if the configuration is correct before applying any changes.
- **`github.com/Masterminds/sprig`**: This library provides a

set of additional functions for Go templates, extending the functionality of the standard template engine. It’s used for template rendering, which allows the project to handle dynamic configuration files with more complex logic, such as string manipulations and formatting.

- **`k8s.io/client-go`**: This is the Kubernetes client library, and it is used for interacting with the Kubernetes API. It is critical for managing resources like secrets, config maps, and clusters. The code in `pkg` and `internal` uses this library to interact with a Kubernetes cluster, fetch resources, and apply changes based on the configurations provided.

These libraries are essential for handling command-line interfaces, logging, validation, templating, and Kubernetes interactions. Their usage is relatively intuitive, and you can see their application by following the relevant sections in the codebase. For instance, `cobra` is mainly in `cmd`, `logrus` is used for error and event logging throughout, and `jsonschema` is utilized in validating the configuration YAML.

</details>

---

### **How does caching work, and where can the caching behavior be modified in the code?**

<details>
<summary>Answer</summary>

Caching is implemented directly in the file download process. Every file downloaded from an external source is saved in the local cache directory, which resides within the project’s configuration folder (`.furyctl/cache`). The cache helps avoid downloading the same files again, improving performance and reducing reliance on external connections.

The code that handles this functionality can be found in `pkg/dependencies/download.go` at line 43, where the files are downloaded, and in `pkg/x/net/client.go` at line 65, where caching is managed. If you want to modify the caching behavior, you can intervene on these files to add custom logic, such as version validation or timestamp checks to determine when to update the cache.

</details>

---

### **How do you use `furyctl` in airgapped environments, and what flags are useful in these cases?**

<details>
<summary>Answer</summary>

In airgapped environments, where no external connection is available to download binaries and dependencies, everything must be pre-downloaded and available locally. Binaries and resources can be copied manually into the `.furyctl` folder via tools like Ansible or committed directly to the project.

To use `furyctl` in these environments, the following flags should be used:

- `--distro-location`: This flag specifies the local path of the downloaded distribution, allowing `furyctl` to use the local version instead of attempting to download it.
- `--skip-deps-download`: This flag skips downloading additional dependencies or binaries from external sources, ensuring that everything is used from the cache or the local distribution.

The air-gapped feature is documented here: https://docs.kubernetesfury.com/docs/advanced-use-cases/air-gapped.

</details>

---

### **How does `furyctl` apply patches to distribution versions, and does it download new dependency versions or use the initial ones?**

<details>
<summary>Answer</summary>

The patches are applied in a way that resembles a "copy-paste" over the downloaded distribution files. When a patch (e.g., for `kfd.yaml`) is provided, it is applied directly on top of the version of the distribution already downloaded and available on the system. Before applying the patch, no new dependency versions are downloaded; instead, the initial version (the one downloaded initially) is used, and the specified changes are overwritten on top.

This approach is helpful for applying local customizations without needing to repeat the entire dependency download process.

</details>

---

### **How does `furyctl` validate the schema of the `furyctl.yaml` file, what is done by our library, and what is handled by the official library?**

<details>
<summary>Answer</summary>

The schema of the `furyctl.yaml` file is validated using the Go library `https://github.com/santhosh-tekuri/jsonschema`. This library allows for validating a JSON/YAML file against a defined schema, ensuring that the structure of the data is correct and conforms to the specifications.

Our library does not directly intervene in this validation step but merely downloads and provides the correct schema via `fury-distribution`, which is then used for the validation process.

</details>

---

### **Are there any coding standards in place? For example, do we use structs with methods instead of functions?**

<details>
<summary>Answer</summary>

The project follows the typical Go coding standards. Specifically, structs with associated methods are preferred over using standalone functions. This helps encapsulate business logic better and makes the code more organized and maintainable. Additionally, Go’s naming and formatting conventions are followed, such as using lowercase letters for private variables and methods.

</details>

---

### **How to set up a local development environment, using the Makefile?**

<details>
<summary>Answer</summary>

To set up a local development environment, it is recommended to use an editor that supports Go debugging, such as Visual Studio Code or GoLand. The Makefile includes several useful commands for the development environment:

- `make tools`: Installs the necessary dependencies for the project.
- `make format-go`: Runs Go formatting, useful if you don't have automatic formatting enabled in the editor.
- `make license-add`: Adds a license to newly added files.
- `make lint-go`: Runs Go linting to check for style issues or common errors.
- `make test-unit`: Runs unit tests.
- `make test-integration`: Runs integration tests.

Other commands in the Makefile are less relevant during the development cycle, so these are the main ones to use.

</details>

---

### **What's the release process for a new version?**

<details>
<summary>Answer</summary>

The release process for a new version is documented at [this link](https://github.com/sighupio/fury-distribution/blob/main/MAINTENANCE.md#furyctl). If the release is not tied to `fury-distribution`, it's enough to create a tag and release it. However, if the release is dependent on new versions of the distribution, the process may be more complex and require updating `fury-distribution` before releasing a new version.

</details>

---

### **How does the template engine works and what are the available features?**

<details>
<summary>Answer</summary>

The template engine used is the standard Go template engine, which also leverages the `https://github.com/Masterminds/sprig` library. Sprig provides several additional functions for templates, such as string manipulations, date formatting, and other utilities not included in Go's native template engine.

We've added `toYaml`, `fromYaml` and `hasKeyAny` custom functions to the template engine (`pkg/template/model.go:74`). All files with `.tpl` extension are processed by the template engine, the generated files folder structure remains the same and the file is simply renamed without the `.tpl` extension (for example `apply.sh.tpl` to `apply.sh`). The folder processed by the template engine is different depending on the phase, for example for `distribution` the folder is taken from the fury-distribution downloaded by furyctl path `templates/distribution`.

</details>

---

### **How logging is implemented, libraries used, and conventions (e.g., log messages)?**

<details>
<summary>Answer</summary>

Logging in the project is implemented using the `https://github.com/sirupsen/logrus` library, which is one of the most popular logging libraries for Go. There are no strict conventions for log messages, but Logrus supports various logging levels (info, error, debug) and can output logs in different formats, including plain text and JSON.

The logs of all the tools used by furyctl, such as Terraform and Ansible, are intercepted and displayed using Logrus. They are also written to the furyctl log file. To display all logs when using furyctl use the flag `--debug`.

</details>

---

### **Is there any best pratice in place for logging?**

<details>
<summary>Answer</summary>

- Log messages that the user sees by default should provide useful information and not leak implementation details, for example:

  BAD:
  INFO Running ansible playbooks

  GOOD:
  INFO Installing Kubernetes packages in the nodes

- All the tools that we call should be configured to output structured logs and should be wrapped in furyctl structured logs in the log file. This is handled automatically by the tools implementation on furyctl.

</details>

---

### **Are there any critical parts of the project that require special attention during future development?**

<details>
<summary>Answer</summary>

There are no critical parts of the project that require immediate attention. However, the code that create the various folders where it copies the templates can be re-engineered and simplified. Also reducing code duplication would help future development by making the code easier to maintain and extend.

</details>

---

### **What's the difference between `{file://}` and `{path://}`, when to use one or the other, and what's the rationale behind their implementation?**

<details>
<summary>Answer</summary>

- `{file://}`: This schema is used to load the content of a file as a string in the `furyctl.yaml`. When you use `{file://}`, the actual content of the file is read and embedded directly in the configuration file.

- `{path://}`: This schema resolves a path relative to the `furyctl.yaml` file and turns it into an absolute path. This is useful when you need to refer to a file relative to the configuration file but want to ensure that the path is always resolved correctly.

An example of using `{path://}` is when you need to specify a file path inside a string in `furyctl.yaml`, for instance, as part of an URL or a complex configuration.

</details>

---

### **Are there any known issues or bugs that are still open and for which workarounds are used?**

<details>
<summary>Answer</summary>

There are no known major bugs or workarounds at this time.

</details>

---

### **What is an _upgrade path_?**

<details>
<summary>Answer</summary>

It is a set of instructions for _furyctl_ in order to perform an upgrade between two versions. As many other components of _furyctl_, the instructions to perform an upgrade are contained in one or multiple templated bash scripts. Every bash script is run as a hook in one of the _phases_ of the install process.

</details>

---

### **How to write *upgrade path*s?**

<details>
<summary>Answer</summary>

You should create a new file under `config/upgrades/{onpremises,kfddistribution,ekscluster}/{starting-version}-{target-version}/hook.tpl`, where `{starting-version}` and `{target-version}` are two different SKD versions.

In your typical _upgrade path_ there will be a file named `pre-distribution.sh.tpl` which will disable admission webhooks in order not to create problems during the deploy. Don't worry, there's no need to restore them as they will be reprovisioned later in the install process!

In the OnPremises upgrade paths when there are Kubernetes version upgrades you also need to include a `pre-kubernetes.sh.tpl` file to run the Ansible playbook that upgrade control planes and worker nodes (for example `configs/upgrades/onpremises/1.29.5-1.30.0/pre-kubernetes.sh.tpl`). This usually only happens during Kubernetes minor version bumps (for example `1.29.5` to `1.30.0`) but there are some exceptional cases where we upgrade the Kubernetes version in a patch release (for example `1.29.4` to `1.29.5`).

</details>
