# Email Server RoboGopher

## Table Of Contents

- [Setup](#setup)

## Setup

- <https://www.digitalocean.com/community/tutorials/how-to-build-and-install-go-programs>

- postgresql dsn string formats: https://stackoverflow.com/questions/3582552/what-is-the-format-for-the-postgresql-connection-string-url

## Commands

- Run email server: `go run .`

### Docker

Build Docker image:

```bash
docker build --tag docker-email-server .
```

Run Docker container:

```bash
docker run docker-email-server --config=config.json --logdir=./logs
```

Server listens on port 25 by default. Connect using `--network=host.docker.internal` for host machine localhost and `--network=network-name` for Docker bridge network.

Map ports with `--publish host-port:target-port`.

Listen on `host.docker.internal` for local testing, on `0.0.0.0` when deployed.

Mount configuration file: `--mount type=bind,source="path/to/go-email-server/config.json",target=/config.json`

Mount logs folder: `--mount type=bind,source="path/to/logs",target=/logs`

Source is in the host machine, target is in the container.

## Logs

Log levels are: `debug`, `info`, `error`, `warn`, `fatal`, `panic`


