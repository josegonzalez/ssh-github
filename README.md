# ssh-github

An ssh server with support for ssh via a github user's keys

## requirements

golang 1.11+

## building

```shell
make build
```

## usage

There is a single required environment variable, `GITHUB_USER`. This should point to your github username.

```shell
export GITHUB_USER=josegonzalez
./ssh-github
```

This will import the ssh keys for the user `josegonzalez`, run the server on port `2222`, and start accepting connections. Any user with valid private keys for the github `josegonzalez` may now authenticate to the server.

You may specify the port via the `PORT` environment variable.

```shell
export GITHUB_USER=josegonzalez
export PORT=2200
./ssh-github
```

You may enforce that the authenticating username matches the specified `GITHUB_USER` by setting `CHECK_GITHUB_USER` to `true`.

```shell
export CHECK_GITHUB_USER=true
export GITHUB_USER=josegonzalez
./ssh-github
```

The default entrypoint is `/bin/bash`, but you can override this via the `SSH_ENTRYPOINT` environment variable to another binary. 

> The ssh server will fail to start if the entrypoint is not a valid binary file. It will also verify this when accepting connections, though will not crash if the binary is missing.

```shell
export GITHUB_USER=josegonzalez
export SSH_ENTRYPOINT=/usr/local/bin/fish
./ssh-github
```

By default, the entrypoint will be executed as the `user:group` that invokes `ssh-github`. This may be overriden via the `SSH_USER_ID` and `SSH_GROUP_ID` environment variables:

```shell
export GITHUB_USER=josegonzalez
export SSH_GROUP_ID=20
export SSH_USER_ID=501
./ssh-github
```

Every invocation of the server starts with a new host key. You may specify the path to a host key via the `HOST_KEY_FILE` environment variable.

```shell
export GITHUB_USER=josegonzalez
export HOST_KEY_FILE="/path/to/host/key"
./ssh-github
```

You may specify multiple host key files by delimiting them via a `:` character.

```shell
export GITHUB_USER=josegonzalez
export HOST_KEY_FILE="/path/to/host/key:/path/to/another/host/key"
./ssh-github
```
