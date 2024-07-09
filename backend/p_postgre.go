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

// let's start with setting up a connection with a database

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	dbname := os.Getenv("DB_NAME")
	dbuser := os.Getenv("DB_USER")
	dbsecret := os.Getenv("DB_SECRET")

	connectToDb(dbname, dbuser, dbsecret)
}

func connectToDb(name string, user string, secret string) {
	// define connection string with db name, user and password
	connStr := fmt.Sprintf("dbname=%s user=%s password=%s", name, user, secret)
	// connect to db
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Logged in successfully")

	// close db connection once everything is done
	// defer db.Close()

	// Ping verifies a connection to the database is still alive, establishing a connection if necessary
	// db.Ping()
	// if err != nil {
	//	log.Fatal(err)
	//}
}

// let's write a proper ass processor
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
func PSQLProcessor() backends.Decorator {
	var (
		config *psqlConfig
		db     *sql.DB
	)

	// init function loading config file
	backends.Svc.AddInitializer(backends.InitializeWith(func(backendConfig backends.BackendConfig) error {
		configType := backends.BaseConfig(&psqlConfig{})
		bcfg, err := backends.Svc.ExtractConfig(backendConfig, configType)
		if err != nil {
			return err
		}
		config := bcfg.(*psqlConfig)
		return nil
	}))

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

					// call the next processor in the chian
					return p.Process(e, task)
				}
				return p.Process(e, task)
			},
		)
	}
}
