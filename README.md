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

todo

- document the steps performed in the PSQL processor and quickly explain PSQL methods
- document the steps performed in the MIME parser
- in thesis mention how processor decoration is done with ProcessorConstructor
- revisit all comments in files and convert them into documentation
- briefly explain default processor functionalities
- document database connecting
- document database queries performed
- document future improvements

## Processors

### Developing and extending

Go Guerilla documentation: [Backends, configuring and extending](https://github.com/phires/go-guerrilla/wiki/Backends,-configuring-and-extending).

the base implementation of the processor can be extended using the decorator pattern as suggested by developers (link to documentation). the idea is that we take the processor as parameter, add some code extending the functionality, and execute the processor before or after our custom code. This approach allows us to add functionality on top of the base implementation without breaking the base implementation.

each worker is structured using a decorator pattern. the decorator pattern chains the processors into a single processor. each processor gets the envelope as the pointer and a task, processes the envelope based on the task, and then either passes the envelope to the next processor in the chain or returns the result back to the caller.

#### `processor.go`

`Processor` is defined to have a method that takes in the [Guerilla `Envelope`](https://github.com/phires/go-guerrilla/blob/master/mail/envelope.go) and a `SelectTask` task, processes it, and returns the `Result` result and error if such arose.

```go
type Processor interface {
  Process(*mail.Envelope, SelectTask) (Result, error)
}

```

So we got `Processor` interface that serves as the base for all processor implementations. `ProcessWith` is a function type with a function signature matching the `Processor` interface. `Process` is implemented to act as a wrapper for the `ProcessWith` by calling the underlying function `f` with parameters passed to Process and returns the result.

```go
// Signature of Processor
type ProcessWith func(*mail.Envelope, SelectTask) (Result, error)

// Make ProcessWith will satisfy the Processor interface
func (f ProcessWith) Process(e *mail.Envelope, task SelectTask) (Result, error) {
  // delegate to the anonymous function
  return f(e, task)
}
```

so yeah there is `DefaultProcessor` that is an undecorated worker that does nothing. Its `Process` method just returns the Result. It's the last processor in the stack, so if it's reached then all is good.

```go
type DefaultProcessor struct{}

func (w DefaultProcessor) Process(e *mail.Envelope, task SelectTask) (Result, error) {
  return BackendResultOK, nil
}
```

#### `decorate.go`

`Decorator` is defined as a function that takes in the `Processor` and returns the `Processor`. `Decorate` function takes in the Processor and a list of `Decorator` decorators. `ds` is a variadic parameter that can allows any number of arguments to be taken as the parameter. so the function takes the processor and then loops through the decorators and for each `decorator` it takes the latest `Processor` stored in `decorated` and wraps it in the `Decorator` function and saves the resulting `Processor` as `decorated`. so yeah that's how the stack is built.

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
```

A processor is registered by saving the name of the processor and corresponding decorator in the `processors` map.

```go
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
```

The actual decoration is done by the backend gateway. it fetches the `processor` map from the configuration file, saves the `Decorator` functions used in the process, and applies them in reverse order to the `DefaultProcessor` using `Decorate` function.

```go
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

This leads us into the discussion about creating your own process. the processor should be a function that returns a `Decorator.` when extending the backend, i am using go guerilla as a package, hence the `backends.Decorator` usage.

ok so line by line. on line 5 `MimeParserProcessor` is defined as a function that returns a `backends.Decorator` (which was defined earlier as `type Decorator func(Processor) Processor`). `MimeParserProcessor` acts as a constructor for the anonymous function.

line 6 we have another anonymous function that accepts the `Processor` as an argument and returns `Processor`. It essentially implements the `Processor` interface with additional logic and creates a new `Processor` that uses `p`.

line 7 we have `ProcessWith` type that is an implementation of the `Processor` interface. It takes an anonymous function defined on lines 8-13 that matches the signature of the `Processor` interface. Essentially, `ProcessWith` is used to construct the decorator with the `Processor` interface. on line 8, you can see it takes arguments envelope as a pointer and a task and returns a result and an error. This function that starts on line 8 will actually do the custom processing of the Envelope as seen on line 9-11. After performing actions on the `Envelope`, the decorated `Processor` `p` is called on line 12 with the processed envelope and the task. this continues the processing sequence and completed the decorator.

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

`Envelope` has an implementation of `io.Reader()` that returns a reader  for reading the delivery headers and the email message. I get the reader for the processed email and pass it to a `ReadEnvelope()` that parses the contents of the provided reader into an `enmime.Envelope`, populating it with email message data, and then insert the `enmime.Envelope` value with ley `envelope_mime` to the `guerilla.Envelope`'s `Values` map. `Values` stores the values generated by the backend processors while processing the envelope.

`enmime.Envelope` stores the plain text portion of the message as `Text` attribute, html portion as `HTML`, attachments, inlines and other parts as their respective slices of `*Part`s. `Part` is a representation of the part in the MIME multipart message. If no plain text is found in the message, the text is extracted from the HTML part of the email message as saved as `Text`. `enmime.Envelope` also has attribute `Root` that stores the headers of the email message.

```go
envReader := e.NewReader()
env, err := enmime.ReadEnvelope(envReader)
if err != nil {
  p_mime.logger.Warn("Failed to parse email to MIME envelope.")
}
e.Values["envelope_mime"] = env
```

### PSQL processor

This processor extracts the previously parsed data from the `guerilla.Envelope` envelope and saves it as a record in the PSQL database.

the decoration is done the same way as in mime parser processor with a few additional things. see, we need to connect to the database to read and write data and close the connection once we're done. opening and closing it for every envelope is inefficient, so let's set up the connection when the processor is initialized and close it on processor shutdown.

the initializer is the function that is executed when the backend is initialized by the gateway. similarly, shutdowner is a function that is executed when the backend is being shut down by the gateway.

to add the initializer to the backend, function `backends.Svc.AddInitializer` is called. it adds a function that implements `processorInitializer` interface. so yeah we have the `backends.InitializeWith()` anonymous function for that. it wraps anonymous function that matches the `Initialize` function signature. This function is passed the configuration object containing data from the configuration file, which allows the initializer function to access the parameters defined in the configuration file. this same function contains database intiialization.

```go
type processorInitializer interface {
  Initialize(backendConfig BackendConfig) error
}

type InitializeWith func(backendConfig BackendConfig) error

// Satisfy ProcessorInitializer interface
// So we can now pass an anonymous function that implements ProcessorInitializer
func (i InitializeWith) Initialize(backendConfig BackendConfig) error {
  // delegate to the anonymous function
  return i(backendConfig)
}

// AddInitializer adds a function that implements ProcessorShutdowner to be called when initializing
func (s *service) AddInitializer(i processorInitializer) {
  s.Lock()
  defer s.Unlock()
  s.initializers = append(s.initializers, i)
}
```

```go
var PSQLProcessor = func() backends.Decorator {
  backends.Svc.AddInitializer(backends.InitializeWith(func(backendConfig backends.BackendConfig) error {
    // loading configuration, setting up database connection
  }))
  // the rest of the processor
}
```

Shutdowner works about the same, except the function is executed during the backend shutdown and it doesn't require the `backends.BackendConfig`.

```go
type processorShutdowner interface {
  Shutdown() error
}

type ShutdownWith func() error

// satisfy ProcessorShutdowner interface, same concept as InitializeWith type
func (s ShutdownWith) Shutdown() error {
  // delegate
  return s()
}

// AddShutdowner adds a function that implements ProcessorShutdowner to be called when shutting down
func (s *service) AddShutdowner(sh processorShutdowner) {
  s.Lock()
  defer s.Unlock()
  s.shutdowners = append(s.shutdowners, sh)
}
```

```go
var PSQLProcessor = func() backends.Decorator {
  backends.Svc.AddShutdowner(backends.ShutdownWith(func() error {
    if db != nil {
      return db.Close()
    }
    return nil
  }))
  // the rest of the processor
}
```


```go

```
