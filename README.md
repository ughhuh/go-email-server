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

- in thesis mention how processor decoration is done with ProcessorConstructor
- revisit all comments in files and clean up docstrings
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

An initializer can be registered using `backends.Svc.AddInitializer` function that registers any function that implements `processorInitializer` interface. This function will be executed when the backend is initialized. `backends.InitializeWith` type implements the `processorInitializer` interface and its `Initialize()` method just calls the `InitializeWith` function. Passing the anonymous function with initialization logic to `backends.IntializeWith` ensures the function meets the initializer requirements and can utilize backend configuration during its execution.

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

Shutdown acts as a sister function to the initializer and mirrors its definition and execution logic.

Onto next thing: the email saving. so first i get the data from the envelope that was inserted by the previous processors. so yeah as you can see i fetch stuff like message id, from, to, reply to, sender, recipients, return path, subject from the guerilla envelope. notably, all of these are headers. i verify the validity of the headers using `getAddressesFromHeader` method, which fetches the address slice from the `Header` field of the guerilla `Envelope` and attempts to parse each of them into a RFC5322 email address.

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

so yeah next i check if the mime parser was executed. if yes, i fetch email's content type and corresponding type. only the plain text and html parts of the enmime `Envelope` are saved. if you wish to extend the service to save attachments and other media to the database, then you can retrieve the `enmime.Part`s from the `env_mime` variable set on the line 2.

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

after all the data is fetched from the `guerilla.Envelope`, i create aan empty slice and append values to it in the order that they're written to the database.

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

so since the recipient and to fields in the database aren't set as foreign keys, i need to perform the check manually to ensure that the recipient is a valid user, and that the at least one of the To addresses is a valid user. i recall you need to separate the checks since the recipient can be registered if the smtp message is sent over the localhost. in short, i fetch email addresses from the users table, save them as a slice, and then find the intersection between the slices by trying to fetch each `recipient` from the database email list, and if the action succeeds, i save the recipient in the `set` slice. if the slice isn't empty at the end, then there must be at least one valid recipient.

then i prepare a rather simple insert query `INSERT INTO %s("message_id", "from", "to", "reply_to", "sender", "subject", "body", "content_type", "recipient", "ip_addr", "return_path") VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)` in `prepareInsertQuery()`. if you decide to change the columns in the emails table, then this query needs to be updated. use `$X` syntax to ensure the safe insertion of the data into the query. then the email is written to the database. an entry is then created in the inbox table to link the email message to the user receiving it.

so, the thing is that when an email is sent to 2 addresses on the same server, it is received as one email with 2 recipients. Example log: `"Mail from: alena.galysheva@gmail.com / to: [{af2db5b2-c743-4442-a7d8-c74afaf26907 vm4408.kaj.pouta.csc.fi [] [] false false <nil>  false} {tester vm4408.kaj.pouta.csc.fi [] [] false false <nil>  false}]"`. this is why we have the middle table to associate the email with two valid recipients. this is why the recipients needs to be verified, and the record should be created for each. there can also be recipients to like other services but that's okay since the getValidRecipients returns the ones present in the database and doesn't raise error as long as there's at least 1 valid recipient present.

10:15-13:15, 14:15-16:30, 17:00-18:00
