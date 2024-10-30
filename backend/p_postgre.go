package backend

// imports

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"io"

	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/lib/pq"
	"github.com/phires/go-guerrilla/backends"
	"github.com/phires/go-guerrilla/log"
	"github.com/phires/go-guerrilla/mail"
)

// config instructions: https://github.com/phires/go-guerrilla/wiki/Backends,-configuring-and-extending#extending

// use initializer to open DB connection: https://github.com/phires/go-guerrilla/wiki/Backends,-configuring-and-extending#processor-initialization

// maybe use shutdown to terminate DB connection: https://github.com/phires/go-guerrilla/wiki/Backends,-configuring-and-extending#processor-shutdown

// processor to save emails to postgresql db
// input: header, mailfrom, subject
// output: idk, saves email i guess

// let's write a proper ass processor
type PSQLProcessor struct {
	config *psqlConfig // config entity
	cache  *sql.Stmt   // a struct to store a prepared statement and execute it when needed
	logger log.Logger  // logger
}

type psqlConfig struct {
	Table       string `json:"mail_table"`
	PrimaryHost string `json:"primary_host"`
}

// processor initializer function
type ProcessorIntiitializer interface {
	Initialize(backendConfig backends.BackendConfig) error
}

// processor shutdown function
type ProcessirShutdowner interface {
	Shutdown() error
}

// The PSQLProcessor decorator [save emails to database]
func PSQL() backends.Decorator {
	var (
		config *psqlConfig // config entity
		db     *sql.DB     // database instance (i think)
		vals   []interface{}
	)

	p_psql := &PSQLProcessor{
		logger: backends.Log(),
	}

	// init function loading config file
	backends.Svc.AddInitializer(backends.InitializeWith(func(backendConfig backends.BackendConfig) error {
		configType := backends.BaseConfig(&psqlConfig{})
		bcfg, err := backends.Svc.ExtractConfig(backendConfig, configType)
		if err != nil {
			return err
		}

		config = bcfg.(*psqlConfig)
		p_psql.config = config

		// load env variables
		err = godotenv.Load()
		if err != nil {
			p_psql.logger.Warn("Failed to load ENV variables")
		}

		// connect to database
		db, err = p_psql.connectToDb(os.Getenv("DB_HOST"), os.Getenv("DB_NAME"), os.Getenv("DB_USER"), os.Getenv("DB_SECRET"), os.Getenv("DB_SSLMODE"))
		if err != nil {
			return err
		}
		return nil
	}))

	// if there's a db connection, close it
	backends.Svc.AddShutdowner(backends.ShutdownWith(func() error {
		if db != nil {
			return db.Close()
		}
		return nil
	}))

	return func(p backends.Processor) backends.Processor {
		return backends.ProcessWith(
			func(e *mail.Envelope, task backends.SelectTask) (backends.Result, error) {
				if task == backends.TaskSaveMail {
					// getting values from headers and converting them to strings
					message_id := p_psql.getAddressFromHeader(e, "Message-Id")
					if message_id == "" {
						message_id = p_psql.generateMessageID(config.PrimaryHost)
					}

					from := p_psql.getAddressesFromHeader(e, "From")
					to := p_psql.getAddressesFromHeader(e, "To")
					reply_to := p_psql.getAddressesFromHeader(e, "Reply-To")
					sender := p_psql.getAddressesFromHeader(e, "Sender")
					recipients := p_psql.getRecipients(e)
					return_path := p_psql.getAddressFromHeader(e, "Return-Path")

					subject := e.Subject
					body := p_psql.getMessageBody(e)
					content_type := "text/html"
					if value, ok := e.Header["Content-Type"]; ok {
						content_type = value[0]
					}
					ip_addr := e.RemoteIP
					// let's build a list of values for the query
					vals = []interface{}{} // clean slate
					// add values
					// order: "message_id", "from", "to", "reply_to", "sender", "subject", "body", "content_type", "recipient", "ip_addr", "return_path
					vals = append(vals,
						message_id,
						pq.Array(from),
						pq.Array(to),
						pq.Array(reply_to),
						pq.Array(sender),
						subject,
						body,
						content_type, // todo: add parsing to determine content type if none is present
						pq.Array(recipients),
						ip_addr,
						return_path,
					)
					//for i, v := range vals {
					//	log.Printf("Index %d: Value %v (Type %T)\n", i, v, v)
					//}
					// get a list of to and recepients
					// check if any in database

					db_recepients, err := p_psql.getValidRecepients(db, append(to, recipients...))
					// if not return backends error invalid recepient
					if err != nil {
						return backends.NewResult("403 Error: invalid recepient"), err
					}
					// prepare query
					stmt := p_psql.prepareInsertQuery(db)
					// execute query
					err = p_psql.executeQuery(stmt, &vals)
					if err != nil {
						return backends.NewResult("554 Error: could not save email"), err
					}
					// idk how but get new email id and create inbox entry for each valid recepient
					for _, v := range db_recepients {
						p_psql.insertInboxEntry(db, v, message_id)
					}
					// call the next processor in the chain
					return p.Process(e, task)
				}
				return p.Process(e, task)
			},
		)
	}
}

func (p_psql *PSQLProcessor) connectToDb(host string, name string, user string, secret string, sslmode string) (*sql.DB, error) {
	// define connection string with db name, user and password
	connStr := fmt.Sprintf("host=%s dbname=%s user=%s password=%s sslmode=%s", host, name, user, secret, sslmode)
	// connect to db
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	p_psql.logger.Info("Connected to database.")
	return db, err
}

func (p_psql *PSQLProcessor) prepareInsertQuery(db *sql.DB) *sql.Stmt {
	insertQuery := `INSERT INTO %s("message_id", "from", "to", "reply_to", "sender", "subject", "body", "content_type", "recipient", "ip_addr", "return_path") 
	VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	// add query to stmt
	cache, err := db.Prepare(fmt.Sprintf(insertQuery, p_psql.config.Table))
	if err != nil {
		p_psql.logger.Fatal(err)
	}
	p_psql.cache = cache
	return cache
}

func (p_psql *PSQLProcessor) executeQuery(cache *sql.Stmt, vals *[]interface{}) error {
	p_psql.logger.Debug("Executing query with values %v", vals)
	_, err := cache.Exec(*vals...)
	if err != nil {
		p_psql.logger.Warn("Failed to write data to the database: %s", err)
	}
	return err
}

func (p_psql *PSQLProcessor) getAddressFromHeader(e *mail.Envelope, headerKey string) string {
	value, ok := e.Header[headerKey]
	if ok {
		// so, why this? e.Header is set by ParseHeaders(): e.Header, err = headerReader.ReadMIMEHeader()
		// type MIMEHeader map[string][]string from net/textproto
		// i can just fetch the data directly from the e.Header, but it hasn't been validated in any way
		// NewAddress checks the validity of the email address
		// it also kinda works for message-id since its format is <unique.id@domain.com>
		// and func (a *Address) String() string does the heavy lifting of finding whatever field has value and converting it to string
		// Alternatively, I can just make my own regex format checkers without extra overhead and objects
		// TODO: consider writing validators instead of using Address, and add length checks there too
		addr, err := mail.NewAddress(value[0])
		if err != nil {
			return ""
		}
		return addr.String()
	}
	return ""
}

// handle multiple addresses
func (p_psql *PSQLProcessor) getAddressesFromHeader(e *mail.Envelope, headerKey string) []string {
	values, ok := e.Header[headerKey]
	if ok {
		var addresses []string
		for _, value := range values {
			addr, err := mail.NewAddress(value)
			if err != nil {
				continue
			}
			addresses = append(addresses, addr.String())
		}
		return addresses
	}
	return nil
}

func (p_psql *PSQLProcessor) getRecipients(e *mail.Envelope) []string {
	var recipients []string
	for _, rcpt := range e.RcptTo {
		recipients = append(recipients, rcpt.String())
	}
	return recipients
}

// there literally isn't a good way to get this shit to work with guerilla
// so yeah fingers crossed i hope it works
func (p_psql *PSQLProcessor) getMessageBody(e *mail.Envelope) string {
	bodyReader := e.NewReader()
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		p_psql.logger.Warn("Failed to read email body.")
		return ""
	}
	return string(body)
}

// borrowed from https://github.com/juliangruber/go-intersect/blob/master/intersect.go
func (p_psql *PSQLProcessor) getValidRecepients(db *sql.DB, recepients []string) ([]string, error) {
	// get list of recepients
	// get list of inboxes from db
	rows, err := db.Query(`SELECT email_address FROM users;`)
	if err != nil {
		p_psql.logger.Warnln("Failed to fetch inboxes from database: %s", err)
	}
	var inboxes []string
	for rows.Next() {
		var email string
		err := rows.Scan(&email)
		if err != nil {
			p_psql.logger.Fatalln(err)
		}
		inboxes = append(inboxes, email)
	}
	// find overlap
	set := make([]string, 0)
	hash := make(map[string]struct{})
	for _, v := range recepients {
		hash[v] = struct{}{}
	}
	for _, v := range inboxes {
		if _, ok := hash[v]; ok {
			set = append(set, v)
		}
	}
	if len(set) == 0 {
		return nil, DatabaseError("No valid email recepients found.")
	}
	return set, nil
}

func (p_psql *PSQLProcessor) insertInboxEntry(db *sql.DB, email string, message_id string) {
	query := `INSERT INTO inboxes (user_id, mail_id) VALUES ($1, $2)`
	_, err := db.Exec(query, email, message_id)
	if err != nil {
		p_psql.logger.Warnf("Failed to create inbox entry! message_id %[1] for email %[2]", message_id, email)
	}
}

// yeah i stole it and i don't care: https://github.com/emersion/go-message/blob/v0.18.1/mail/header.go#L338
func (p_psql *PSQLProcessor) generateMessageID(hostname string) string {
	now := uint64(time.Now().UnixNano())
	nonceByte := make([]byte, 8)
	nonce := binary.BigEndian.Uint64(nonceByte)
	message_id := fmt.Sprintf("%s.%s@%s", base36(now), base36(nonce), hostname)
	return message_id
}

// https://github.com/emersion/go-message/blob/v0.18.1/mail/header.go#L352
func base36(input uint64) string {
	return strings.ToUpper(strconv.FormatUint(input, 36))
}
