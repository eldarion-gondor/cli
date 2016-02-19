# ChangeLog

## 0.8.1

### Bug fixes

* fixed error handling when loading global and site configuration
* global config learned to create ~/.config when it does not exist
* fixed persistence of identities when credentials are revoked

## 0.8.0

This marks the first release of the CLI base library. The previous release of
this code was tied to the gondor command-line tool v0.7.1. The changes listed
here are an continuation of that release.

### Backward Incompatible Changes

#### Support for custom clouds and clusters

It is now possible to configure the CLI to target custom cloud and clusters. To do this the CLI will look for `~/.config/gondor/clouds.json`. An example configuration might be:

    {
        "clouds": [
            {
                "name": "my-cloud",
                "identity": {
                    "type": "oauth2",
                    "location": "identity.example.com",
                    "client-id": "..."
                },
                "current-cluster": "my-cluster",
                "clusters": [
                    {
                        "name": "my-cluster",
                        "location": "api.my-cluster.example.com",
                        "insecure-skip-verify": true
                    }
                ]
            }
        ]
    }

To support this feature, identity management has been centralized to
`~/.config/gondor/identities.json`. This is required to share credentials
between clusters within the same cloud. An example `identities.json` file:

    {
        "identities": [
            {
                "provider": "identity.example.com",
                "username": "my-username",
                "oauth2": {
                    "access_token": "...",
                    "refresh_token": "..."
                }
            }
        ]
    }

### Features

* cli learned `--cloud` and `--cluster` flags to indicate which cloud and cluster to target

### Bug fixes

* `services env|restart` learned `--instance` flag
* removed more instances of `gondor` and `g3a` in help output
