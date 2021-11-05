# Welcome to Couchbase Fluent Bit contributing guide <!-- omit in toc -->

In this guide you will get an overview of the contribution workflow from opening an issue, creating a PR, reviewing, and merging the PR.

## New contributor guide

To get an overview of the project, read the [README](README.md). Here are some resources to help you get started with open source contributions:

- [Set up Git](https://docs.github.com/en/get-started/quickstart/set-up-git)
- [GitHub flow](https://docs.github.com/en/get-started/quickstart/github-flow)
- [Collaborating with pull requests](https://docs.github.com/en/github/collaborating-with-pull-requests)

## Issues

### Create a new issue

If you spot a problem, [search if an issue already exists](https://issues.couchbase.com/secure/RapidBoard.jspa?rapidView=125&view=planning). To filter on just Fluent Bit issues, you can [filter](https://issues.couchbase.com/issues/?jql=project%20%3D%20%22Couchbase%20Kubernetes%22%20AND%20component%20%3D%20logging) on  the `logging` component. If a related issue doesn't exist, you can open a new issue but will need a JIRA account with Couchbase to do so.

### Solve an issue

Scan through our [existing issues](https://issues.couchbase.com/issues/?jql=project%20%3D%20%22Couchbase%20Kubernetes%22%20AND%20component%20%3D%20logging) to find one that interests you.

## Development process
We have regression tests in place using known input with expected output: https://github.com/couchbase/couchbase-fluent-bit/tree/main/test/logs
This directory includes the input as the name of the log with expected output using the same name with a .expected suffix. The names of the log files should match the Couchbase Server ones.

It may be that new edge cases come up for logs where we want to add a new line or file with new expected output. That should just be a case of updating the above. Similarly if you add new features that change the output then you’ll have to update the expected output - this is actually quite good to show how the change would look in "real life".

An example change: https://github.com/couchbase/couchbase-fluent-bit/commit/7ce9d3c625181a9fd123fdb27962309ea00a66e1
This adds a new numeric log level field so you can see the actual changes to the configuration but also the test & documentation updates. We generally keep track of changes in the README.md (it’s also useful then for the release ticket): https://github.com/couchbase/couchbase-fluent-bit#releases

We also strive to include Go unit tests for any functionality in the Go watcher.

Generally everything is covered by a simple make command which builds and runs all the regression tests. You do still need to publish an unofficial container to use for the operator CICD run of logging tests:

1. `make container-public -e DOCKER_USER=<your username> -e DOCKER_TAG=<gerrit SHA>`
2. Go to: https://jenkins.spjmurray.co.uk/job/couchbase-operator-continuous-integration/build?delay=0sec
3. Insert the image from 1 into the settings.
4. Run the `logging` feature tests.
5. Clean up/remove the image you made.

Quite often we will update to a new Fluent Bit version, the easiest way to do this is just to find-and-replace the string `FLUENT_BIT_VER=1.8.9` (or whatever the current version is) with the new value. This should affect the Makefile and Dockerfiles.

We have two container images - the vanilla container just layers on top of the OSS distroless images for Fluent Bit but the Red Hat container has to build it from source with some tweaks. We could use the RPMs for RHEL 7 but that’s not a great idea (there is an example doing this here: https://github.com/couchbase/couchbase-fluent-bit/tree/main/tools/ubi7) for two reasons:
UBI 8 - this requires a huge amount of dependencies to use an OS style RHEL 7 RPM
It is not versioned, you always get the latest at time of build.

There is a lot of supporting tooling as well in https://github.com/couchbase/couchbase-fluent-bit/tree/main/tools
This includes things like compose and K8S stacks for Loki as well as a simple integration tester that allows you to pass any server version.

There are also make targets that do things like confirm it should work with the official build process - these are all run during the normal make all. Do not change any parameters in these unless build make a change otherwise it won’t match. The point of them is to prevent success and block commits that would just fail to build later in an official release.

Be aware that Couchbase build infrastructure uses Docker Swarm so can run containers but it cannot use bind mounts. Hence you will see some workarounds in some of the make targets to deal with this.

## Release process

This process is intended to be used by Couchbase employees only:

1. Raise CBD ticket for release on yourself, e.g. https://issues.couchbase.com/browse/CBD-4444
2. Document what’s in it and whether it is a major or breaking change release - that would be tied to CAO then
3. Assign to QE for verification sign off (Roo) - ask for the versions you want to test against with CAO.
4. Raise K8S ticket for compatibility matrix update and link to release ticket, e.g. https://issues.couchbase.com/browse/K8S-2325
5. Raise at Ops meeting - get someone to add a comment saying it is approved
6. Assign to build - they will do the release which includes auto-tagging in Git. Confirm you can pull it ok and it is right then just close the CBD.
7. Once released, send out an email to the invite list for the Ops meeting
8. Once released, update compatibility matrix - this may need pushing to CAO branches for older releases too but generally not

Generally speaking QE will automatically pick up the latest builds and test them against the latest CAO build. Here we want to explicitly verify it against the latest release plus any previous compatible releases you want to say are supported.

The other aspect to consider for a formal release is license checking via Black Duck but this is the standard Couchbase process. It will be scanned and results are in the usual place so just need reviewing. Ensure you exclude any tooling stuff (e.g. linting, Terrier, etc.).

## Licensing

The Couchbase Fluent Bit component is completely open source and distributed under the terms of the Apache 2.0 license so we need to ensure we stay consistent with this on each release.
The [Couchbase build infrastructure will automatically run Black Duck scans on the source code](https://hub.internal.couchbase.com/confluence/display/CR/Blackduck+Scanning+Process) - not the binary artefacts produced, only the Go code for the watcher process.
Review any licensing changes either via the Black Duck UI or here: https://github.com/couchbase/product-metadata/blob/master/couchbase-fluent-bit/blackduck/<VERSION>/notices.txt
Replace `<VERSION>` with the version being checked.
This will require access and likely some manual review against the dependency graph: some dependencies are build time only for tooling.