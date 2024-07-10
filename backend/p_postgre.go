package renameme

// imports

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/phires/go-guerrilla/backends"
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
}

type psqlConfig struct {
	SomeOption string `json:"maildir_path"`
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
	)
	p_psql := &PSQLProcessor{}

	// init function loading config file
	backends.Svc.AddInitializer(backends.InitializeWith(func(backendConfig backends.BackendConfig) error {
		configType := backends.BaseConfig(&psqlConfig{})
		bcfg, err := backends.Svc.ExtractConfig(backendConfig, configType)
		if err != nil {
			return err
		}
		config := bcfg.(*psqlConfig)

		// load env variables
		err = godotenv.Load()
		if err != nil {
			log.Fatal("Error loading .env file")
		}
		dbname := os.Getenv("DB_NAME")
		dbuser := os.Getenv("DB_USER")
		dbsecret := os.Getenv("DB_SECRET")
		// connect to database
		db, err = p_psql.connectToDb(dbname, dbuser, dbsecret, db)
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
					// use config somewhere here
					// i think i can pass the table name in config as mail_table

					stmt := p_psql.prepareInsertQuery(db)
					err := p_psql.executeQuery(db, stmt, &vals)
					if err != nil {
						return backends.NewResult(fmt.Sprint("554 Error: could not save email")), StorageError
					}
					// call the next processor in the chain
					return p.Process(e, task)
				}
				return p.Process(e, task)
			},
		)
	}
}

func (p_psql *PSQLProcessor) connectToDb(name string, user string, secret string, db *sql.DB) (*sql.DB, error) {
	// define connection string with db name, user and password
	connStr := fmt.Sprintf("dbname=%s user=%s password=%s", name, user, secret)
	// connect to db
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Logged in successfully")
	return db, err
}

func (p_psql *PSQLProcessor) prepareInsertQuery(db *sql.DB) *sql.Stmt {
	insertQuery := `"INSERT INTO $1("message_id", "from", "to", "reply_to", "sender", "subject", "body", "content_type", "recipient", "ip_addr", "return_path") 
	VALUES($2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)"`
	// add query to stmt
	cache, err := db.Prepare(insertQuery)
	if err != nil {
		log.Fatal(err)
	}
	return cache
}

func (p_psql *PSQLProcessor) executeQuery(db *sql.DB, cache *sql.Stmt, vals *[]interface{}) error {
	insertStmt := p_psql.prepareInsertQuery(db)
	_, err := insertStmt.Exec(*vals...)
	if err != nil {
		fmt.Println("Failed to write data to the database: %s", err)
	}
	return err
}
