# ChangeLog

## 0.8.0

This marks the first release of the CLI base library. The previous release of
this code was tied to the gondor command-line tool v0.7.1. The changes listed
here are an continuation of that release.

### Backward Incompatible Changes

* configuration has changed to support multiple clouds and clusters

### Features

* cli learned `--cloud` and `--cluster` flags to indicate which cloud and cluster to target

### Bug fixes

* `services env|restart` learned `--instance` flag
