# MTA Service

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

Database connection parameters are defined as environmental variables.

`.env`:

```sh
DB_HOST=string
DB_NAME=string
DB_USER=string
DB_SECRET=string
DB_SSLMODE=string
```

- `DB_HOST` is the database hostname. Can be set to `localhost`, `host.docker.internal`, or the name of the service when used with Docker compose.
- `DB_NAME` is the name of the database.
- `DB_USER` is the database user to use to execute queries.
- `DB_SECRET` is the password of the database user.
- `DB_SSLMODE` is the database SSL mode. Available values: `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full`.

## Processors

### Developing and extending

Go Guerilla documentation: [Backends, configuring and extending](https://github.com/phires/go-guerrilla/wiki/Backends,-configuring-and-extending).

MTA service was implemented on the base of Go Guerilla daemon. When the service is started, the configuration is loaded and applied it to the daemon, which then started, and the main thread starts to listen for incoming interruption signals to gracefully handle them. The daemon starts the gateway backend and worker processes and proceeds to listen for SMTP traffic on the interface specified in the configuration file. When an SMTP message is received, it is converted to the Go Guerrilla Envelope, added to the conveyor channel, from which the message is picked up by an available worker process. The worker processes the Envelope using the Processor decorated with the processors specified in the configuration file, which typically includes parsing message headers and body, and writing the processed data to the database. On a graceful shutdown, each worker is terminated without force, allowing them to finish their tasks without data loss.

Guerilla daemon is structured around the Gateway backend system, which orchestrates a set of workers running in their own goroutines. When a Gateway receives a new envelope object from the server, it adds it to the conveyor channel, from which it can be picked and processed by an available worker. Each worker is defined as a set of processors that are called in succession to process the envelope. When the processor is done processing the envelope, it should either pass it down the execution chain to the next processor or return the result back to the caller. Once all the processors are executed, the worker sends the results back. As a part of the development work for this thesis, two processors were implemented to process the MIME messages and the save the data of the envelope to a PostgreSQL database and verify the validity of the recipient.

Go Guerilla has a base implementation of the Processor interface that can be extended using decorator pattern. Decorator is a design pattern that allows to add new behavior to a component without affecting its base implementation by wrapping it with another object, known as the decorator. A decorator object implements the same interface as the wrapped (decorated) object, ensuring the client perceives both objects as identical. It forwards requests to the decorated object while performing additional actions before or after the forwarding, preserving the base functionality of the object. Since the decorator holds a reference to the decorated object, it’s possible to design this reference to accept any object that implements the same interface. This compatibility allows multiple decorators to be stacked, combining the effects of their behaviors, while maintaining their independence.

#### `processor.go`

In Go Guerrilla, the `Processor` is implemented as an interface with a `Process` method that takes in an `Envelope` and a `SelectTask`, returning a `Result` and an error. The `ProcessWith` type is a function type with the same signature as the `Process` method, allowing it to act as a wrapper around the base function and enabling the application of decorators.

The `Process` method is defined on the `ProcessWith` type and satisfies the `Processor` interface. Internally, it calls the underlying function `f`, passing in the `Envelope` and `SelectTask`, and returning the result. This design allows `ProcessWith` to be decorated, meaning that additional functionality can be layered on top of the base `Process` method. Decorators can invoke the core processor function either before or after applying their own custom logic, extending functionality without altering the underlying implementation.

```go
type Processor interface {
  Process(*mail.Envelope, SelectTask) (Result, error)
}

// Signature of Processor
type ProcessWith func(*mail.Envelope, SelectTask) (Result, error)

// Make ProcessWith will satisfy the Processor interface
func (f ProcessWith) Process(e *mail.Envelope, task SelectTask) (Result, error) {
  // delegate to the anonymous function
  return f(e, task)
}
```

The `DefaultProcessor` is a simple, undecorated processor that serves as the last step in the processing chain. Its Process method returns a successful `Result` without any additional actions. Since it does not pass the envelope to any subsequent processor, `DefaultProcessor` is placed as the final step in Go Guerilla’s processor stack. When reached, it signals that the envelope has been successfully processed through all preceding layers. Its `Process` method implements the `Processor` interface, which allows the `DefaultProcessor` to be decorated with other Processors.

```go
type DefaultProcessor struct{}

func (w DefaultProcessor) Process(e *mail.Envelope, task SelectTask) (Result, error) {
  return BackendResultOK, nil
}
```

#### `decorate.go`

The `Decorator` type is defined as a function that takes a `Processor` as input and returns a `Processor`. This means any function matching this signature to act as a `Decorator`. To pass the data to the next `Processor` in the stack, each `Decorator` implementation calls the `Process()` method on the `Processor` it receives. The `Decorate` function stacks multiple `Processors` on top of the base `Processor`. It accepts the base `Processor` `c` and a variadic parameter `ds` for an unlimited number of decorators. `Decorate` stores the base Processor as the decorated variable, then iterates through each decorator, wrapping decorated with the current Decorator, and updates decorated each time.  This builds a processing stack, with each decorator layering additional functionality around the previous processor. The function returns the fully decorated `Processor`.

```go
// We define what a decorator to our processor will look like
type Decorator func(Processor) Processor

// Decorate will decorate a processor with a slice of passed decorators
func Decorate(c Processor, ds ...Decorator) Processor {
  decorated := c
  for _, decorate := range ds {
    decorated = decorate(decorated)
  }
  return decorated
}

type ProcessorConstructor func() Decorator

func (s *service) AddProcessor(name string, p ProcessorConstructor) {
  // wrap in a constructor since we want to defer calling it
  var c ProcessorConstructor
  c = func() Decorator {
    return p()
  }
  // add to our processors list
  processors[strings.ToLower(name)] = c
}

func (gw *BackendGateway) newStack(stackConfig string) (Processor, error) {
  var decorators []Decorator
  cfg := strings.ToLower(strings.TrimSpace(stackConfig))
  if len(cfg) == 0 {
    //cfg = strings.ToLower(defaultProcessor)
    return NoopProcessor{}, nil
  }
  items := strings.Split(cfg, "|")
  for i := range items {
    name := items[len(items)-1-i] // reverse order, since decorators are stacked
    if makeFunc, ok := processors[name]; ok {
      decorators = append(decorators, makeFunc())
    } else {
      ErrProcessorNotFound = fmt.Errorf("processor [%s] not found", name)
      return nil, ErrProcessorNotFound
    }
  }
  // build the call-stack of decorators
  p := Decorate(DefaultProcessor{}, decorators...)
  return p, nil
}
```

### MIME parser

The MIME Parser Processor addresses limitations in Go Guerrilla’s default email parsing, which performs basic email header parsing and expects the entire message to be saved in the body. The `MimeParserProcessor` is implemented as a custom processor using the decorator pattern. This processor is set up as a function that returns a `backends.Decorator`, which wraps and extends the functionality of the `Processor` interface.

`MimeParserProcessor` is defined as a function `backends.Decorator`. This function effectively acts as a constructor for the decorator. Inside `MimeParserProcessor`, an anonymous function is defined that takes a `Processor` as an argument and returns a new `Processor`. It acts as a constructor for an anonymous function that will implement additional logic for the `Processor`. `ProcessWith`, which implements the Processor interface, is then initialized on line 7. `ProcessWith` takes an anonymous function that matches the Processor signature, allowing to construct the decorator that conforms to the Processor interface. This function takes the envelope as a pointer and task arguments, performs custom processing, and returns a result and any error. After processing the envelope, it hands the modified envelope off to the next Processor in the chain by calling `p.Process(e, task)`, continuing the processing sequence and completing the decoration.

```go
type MimeParser struct {
  logger log.Logger
}

var MimeParserProcessor = func() backends.Decorator {
  return func(p backends.Processor) backends.Processor {
    return backends.ProcessWith(
      func(e *mail.Envelope, task backends.SelectTask) (backends.Result, error) {
        if task == backends.TaskSaveMail {
          // envelope processing...
        }
        return p.Process(e, task)
      })
  }
}
```

Figure 6 contains the code used to parse the email message by the processor using the `enmime` library. A reader for the email message is retrieved from the Guerilla Envelope object. This reader is then passed to ReadEnvelope(), which processes the contents into an `enmime.Envelope` object. Once populated, the `enmime.Envelope` is stored in the Values map of `guerilla.Envelope` under the key envelope mime, which stores the values generated by the backend processors while processing the envelope. Within `enmime.Envelope`, email content is segmented into attributes that simplify data handling. `Text` attribute holds the plain text version of the message, while HTML stores the HTML version. Other components, like attachments and inline elements, are organized in respective slices of `*Part` objects, where each `Part` represents a distinct section in the MIME multipart message. If no plain text is directly provided, `enmime.Envelope` will extract plain text from the HTML version and store it in the `Text` attribute. `Root` attribute holds the email headers, enabling structured access to the top-level headers of the message.

```go
envReader := e.NewReader()
env, err := enmime.ReadEnvelope(envReader)
if err != nil {
  p_mime.logger.Warn("Failed to parse email to MIME envelope.")
}
e.Values["envelope_mime"] = env
```

### PSQL processor

The PSQL Processor extracts previously parsed email data from the `guerilla.Envelope` object and saves it as a record in the PostgreSQL database. Like the MIME Parser Processor, this implementation uses the decorator pattern, but with additional elements to manage the database connection efficiently. Rather than opening and closing the database connection for each envelope, the connection is established when the processor is initialized and closed when the processor shuts down, which allows the connection to be reused throughout multiple operations.

The initialization is managed by a function registered with `backends.Svc.AddInitializer`, which ensures that any function implementing the `processorInitializer` interface runs when the backend is initialized by the gateway. `backends.InitializeWith`, which implements the `processorInitializer` interface, then calls the provided function with the initialization logic. Similarly, the shutdown process uses a paired shutdown function, which follows the same structure as the initializer to ensure a clean and controlled closure of resources.

```go
var PSQLProcessor = func() backends.Decorator {
  backends.Svc.AddInitializer(backends.InitializeWith(func(backendConfig backends.BackendConfig) error {
    // loading configuration, setting up database connection
  }))
  // the rest of the processor
}
```

```go
type processorInitializer interface {
  Initialize(backendConfig BackendConfig) error
}

type InitializeWith func(backendConfig BackendConfig) error

func (i InitializeWith) Initialize(backendConfig BackendConfig) error {
  // delegate to the anonymous function
  return i(backendConfig)
}

func (s *service) AddInitializer(i processorInitializer) {
  s.Lock()
  defer s.Unlock()
  s.initializers = append(s.initializers, i)
}
```

Once reached in the stack with `TaskSaveEmail` task, the processor handles email saving. The function retrieves relevant headers and metadata from the `guerilla.Envelope`, such as `Message-Id`, `From`, `To`, `Reply-To`, `Sender`, `Recipients`, `Return-Path`, `Subject`, and remote IP and verifies them using the `getAddressesFromHeader` method that extracts and parses each address in compliance with RFC 5322.

```go
  var message_id string
  message_id_header, ok := e.Header["Message-Id"]
  if ok {
    message_id = message_id_header[0]
  } else {
    message_id = p_psql.generateMessageID(config.PrimaryHost)
  }

  from := p_psql.getAddressesFromHeader(e, "From")
  to := p_psql.getAddressesFromHeader(e, "To")
  reply_to := p_psql.getAddressesFromHeader(e, "Reply-To")
  sender := p_psql.getAddressesFromHeader(e, "Sender")
  recipients := p_psql.getRecipients(e)
  return_path := p_psql.getAddressFromHeader(e, "Return-Path")
  ip_addr := e.RemoteIP
  subject := e.Subject
```

Next, if the MIME Parser Processor was executed, the email’s content type, text, and HTML parts are retrieved from the `enmime.Envelope`. Currently, only the plain text and HTML sections are saved. If needed, the processor could be extended to save attachments and other MIME parts by accessing `enmime.Parts` in the parsed envelope.

```go
  var body, content_type string
  env_mime, ok := e.Values["envelope_mime"].(*enmime.Envelope)
  if ok {
    content_type = env_mime.Root.ContentType
    if strings.Contains(content_type, "plain") {
      body = env_mime.Text
    } else if strings.Contains(content_type, "html") {
      body = env_mime.HTML
    }
  } else {
    if value, ok := e.Header["Content-Type"]; ok {
      content_type = value[0]
    }
    body = p_psql.getMessageStr(e) // entire message as default
  }
```

The gathered email data is then organized into a slice for database insertion. Before saving, the processor checks that at least one of the recipients fetched from the envelope is valid by comparing them with entries in the users table and saves the recipients in a slice of strings. The processor proceeds to construct and execute an insert query to add the message to the database, using parameterized syntax to ensure safe data insertion. After the message is saved successfully, an entry is created in the inboxes table for each valid recipient with the message ID of the envelope.

```go
  vals = []interface{}{} // clean slate

  vals = append(vals,
    message_id,
    pq.Array(from),
    pq.Array(to),
    pq.Array(reply_to),
    pq.Array(sender),
    subject,
    body,
    content_type,
    pq.Array(recipients),
    ip_addr,
    return_path,
  )
```
