# Contribution Guide

Welcome to the contribution guide for our project! We appreciate your interest in contributing to the development of this project. Below, you will find essential information on local development, running tests, and guidelines for submitting pull requests.

## Table of Contents

- [🏠🏗️ Local development](#%EF%B8%8F-local-development)
- [🚦 Running Tests](#-running-tests)
- [📖 Submitting PR Guidelines](#-submitting-pr-guidelines)


## 🏠🏗️ Local Development

To run a command locally, use the following command template:

```sh
go run github.com/jfrog/jfrog-cli-platform-services command [options] [arguments...]
```

---

This project heavily depends on the following modules:

- [github.com/jfrog/jfrog-client-go](https://github.com/jfrog/jfrog-client-go)
- [github.com/jfrog/jfrog-cli-core](github.com/jfrog/jfrog-cli-core)

During local development, if you come across code that needs to be modified in one of the mentioned modules, it is advisable to replace the dependency with a local clone of the module.

<details>
<summary>Replacing a dependency with a local clone</summary>

---

To include this local dependency, For instance, let's assume you wish to modify files from `jfrog-cli-core`, modify the `go.mod` file as follows:

```
replace github.com/jfrog/jfrog-cli-core/v2 => /local/path/in/your/machine/jfrog-cli-core
```

Afterward, execute `go mod tidy` to ensure the Go module files are updated. Note that Go will automatically adjust the version in the `go.mod` file.

---

</details>


## 🚦 Running Tests

To run tests, use the following command:

```
./.github/scripts/gotest.sh ./...
```

## 📖 Submitting PR Guidelines

Once you have completed your coding changes, it is recommended to push the modifications made to the other modules first. Once these changes are pushed, you can update this project to resolve dependencies from your GitHub fork or branch.

<details>

<summary>Resolve dependencies from GitHub fork or branch</summary>

---

To achieve this, modify the `go.mod` file to point the dependency to your repository and branch, as shown in the example below:

```
replace github.com/jfrog/jfrog-cli-core/v2 => github.com/jfrog/jfrog-cli-core/v2 dev
```

Finally, execute `go mod tidy` to update the Go module files. Please note that Go will automatically update the version in the `go.mod` file.

---

</details>

### Before submitting the pull request, ensure:

- Your changes are covered by `unit` and `integration` tests. If not, please add new tests.
- The code compiles, by running `go vet ./...`.
- To format the code, by running `go fmt ./...`.
- The documentation covers the changes, if not please add and make changes at [The documentation repository](https://github.com/jfrog/documentation)

### When creating the pull request, ensure:

- The pull request is on the `main` branch.
- The pull request description describes the changes made.