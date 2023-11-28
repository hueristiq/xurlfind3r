# Contributing

Contributions are welcomed!

Please review the following guidelines before contributing. Also, feel free to propose changes to these guidelines by updating this file and submitting a pull request.

* [I have a question...](#have-a-question)
* [I found a bug...](#found-a-bug)
* [I have a feature request...](#have-a-feature-request)
* [I have a contribution to share...](#ready-to-contribute)

## Have a Question?

Please don't open a GitHub issue for questions about how to use `xurlfind3r`, as the goal is to use issues for managing bugs and feature requests. Issues that are related to general support will be closed.

## Found a Bug?

If you've identified a bug in `xurlfind3r`, please [submit an issue](#create-an-issue) to our GitHub repo: [hueristiq/xurlfind3r](https://github.com/hueristiq/xurlfind3r/issues/new). Please also feel free to submit a [Pull Request](#pull-requests) with a fix for the bug!

## Have a Feature Request?

All feature requests should start with [submitting an issue](#create-an-issue) documenting the user story and acceptance criteria. Again, feel free to submit a [Pull Request](#pull-requests) with a proposed implementation of the feature.

## Ready to Contribute

### Create an issue

Before submitting a new issue, please search the issues to make sure there isn't a similar issue doesn't already exist.

Assuming no existing issues exist, please ensure you include required information when submitting the issue to ensure we can quickly reproduce your issue.

We may have additional questions and will communicate through the GitHub issue, so please respond back to our questions to help reproduce and resolve the issue as quickly as possible.

New issues can be created with in our [GitHub repo](https://github.com/hueristiq/xurlfind3r/issues/new).

### Pull Requests

Pull requests should target the `dev` branch. Please also reference the issue from the description of the pull request using [special keyword syntax](https://help.github.com/articles/closing-issues-via-commit-messages/) to auto close the issue when the PR is merged. For example, include the phrase `fixes #14` in the PR description to have issue #14 auto close.

### Styleguide

When submitting code, please make every effort to follow existing conventions and style in order to keep the code as readable as possible. Here are a few points to keep in mind:

* All dependencies must be defined in the `go.mod` file.
	* Advanced IDEs and code editors (like VSCode) will take care of that, but to be sure, run `go mod tidy` to validate dependencies.
* Please run `go fmt ./...` before committing to ensure code aligns with go standards.
* We use [`golangci-lint`](https://golangci-lint.run/) for linting Go code, run `golangci-lint run --fix` before submitting PR. Editors such as Visual Studio Code or JetBrains IntelliJ; with Go support plugin will offer `golangci-lint` automatically.
* For details on the approved style, check out [Effective Go](https://golang.org/doc/effective_go.html).

### License

By contributing your code, you agree to license your contribution under the terms of the [MIT License](https://github.com/hueristiq/xurlfind3r/blob/master/LICENSE).

All files are released with the MIT license.
