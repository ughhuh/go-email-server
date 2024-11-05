# Email Server RoboGopher

## Commands

- Install project dependencies: `go mod download`
- Run the application locally: `go run .`
  - `--config=<file>` where `<file>` is the JSON configuration file with an optional path to it.
- Build the application: `go build .`

### Docker

Build Docker image:

```bash
docker build --tag docker-email-server .
```

Run Docker container:

```bash
docker run docker-email-server --config=config.json
```

Server listens on port 25 by default. Connect using `--network=host.docker.internal` for host machine localhost and `--network=network-name` for Docker bridge network.

Map ports with `--publish host-port:target-port`.

Listen on `host.docker.internal` for local testing, on `0.0.0.0` when deployed.

Mount configuration file: `--mount type=bind,source="path/to/go-email-server/config.json",target=/config.json`

Mount logs folder: `--mount type=bind,source="path/to/logs",target=/logs`

Source is in the host machine, target is in the container.

## Logs

Log levels are: `debug`, `info`, `error`, `warn`, `fatal`, `panic`

## Configuration

Example configuration file:

```json
{
    "allowed_hosts":  ["iot-ticket.com","wapice.com","smtp.pouta.csc.fi","vm4408.kaj.pouta.csc.fi"],
    "log_file" : "./logs/email.log",
    "log_level" : "info",
    "backend_config" :
        {
            "primary_host" : "vm4408.kaj.pouta.csc.fi",
            "log_received_mails" : true,
            "save_process": "HeadersParser|Header|Debugger|MimeParser|PSQL",
            "save_workers_size":  3,
            "mail_table":"emails"
        },
    "servers" : [
        {
            "is_enabled" : true,
            "host_name":"vm4408.kaj.pouta.csc.fi",
            "max_size": 100017,
            "timeout":160,
            "listen_interface":"0.0.0.0:25",
            "start_tls_on":false,
            "tls_always_on":false,
            "max_clients": 2
        }
    ]
}
```

- `allowed_hosts` is the list of hosts to accept emails from.
- `log_file` is where to write log output to. Options: `off`, `stderr`, `stdout`, a path to file.
- `log_level` is the level of logs to register. Options: `debug`, `info`, `error`, `warn`, `fatal`, `panic`.
- `backend_config` is the backend and processor configuration
  - `mail_table` (PSQL) is the table to write emails to in the database
  - `save_process` is the processor chain to execute when saving emails
  - `save_workers_size` is the number of workers to create
- `servers` is the server configuration
  - `host_name` is the hostname of the server as set by MX record
  - `max_size` is the maximum size of email message (bytes)
  - `timeout` is the timeout before an idle connection is closed (seconds)
  - `listen_interface` is the IP and port to listen on
  - `max_clients` is the maximum number of concurrent clients
