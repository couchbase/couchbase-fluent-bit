This directory contains a definition to build a UBI 7 container with the official CentOS 7 binaries of fluent bit included.

Unfortunately there is no control over the version of Fluent Bit we install, the repo just has the latest.

The latest (1.7.5) has a regression issue which means we cannot parse rebalance reports: https://github.com/fluent/fluent-bit/issues/3511