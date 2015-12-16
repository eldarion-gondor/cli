# ChangeLog

## 0.7

This release updates the client to support Gondor 0.2.0. Due to the changes in
Gondor, this release is not fully backward compatible.

### Backward Incompatible Changes

* `gondor.yml` requires a `deploy` config:

  ```
  deploy:
    services:
      - web-[XXXXX]
  ```

  This tells the deploy command which services to target with the build.  You can get the value for the `[XXXXX]` bit by running `g3a services list`.

* `gondor.yml` learned `buildpack` to send to build (replacing `BUILDPACK_URL` environment variable)
* `open` requires a service argument (only works with web services)
* `run` requires a service argument
* `pg` has been removed (use `run`)
* `instances list` no longer shows Web URL

### Features

* `sites init` learned the new `deploy` config for `gondor.yml` generation
* `deploy` learned to deploy to services specified by `deploy` config concurrently

### Bug fixes

* `deploy` honors its source argument