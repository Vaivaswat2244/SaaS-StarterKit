package main

import (
	"encoding/json"
	"expvar"
	"log"
	"net/url"
	"os"

	"geeks-accelerator/oss/saas-starter-kit/internal/platform/flag"
	"geeks-accelerator/oss/saas-starter-kit/internal/schema"
	"github.com/kelseyhightower/envconfig"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	sqltrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
	sqlxtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/jmoiron/sqlx"
)

// build is the git version of this program. It is set using build flags in the makefile.
var build = "develop"

// service is the name of the program used for logging, tracing and the
// the prefix used for loading env variables
// ie: export SCHEMA_ENV=dev
var service = "SCHEMA"

// DB defines the database credentials stored in AWS Secrets Manager as defined by devops.
type DB struct {
	Host       string
	User       string
	Pass       string
	Database   string
	Driver     string
	DisableTLS bool
}

func main() {
	// =========================================================================
	// Logging

	log := log.New(os.Stdout, service+" : ", log.LstdFlags|log.Lmicroseconds|log.Lshortfile)

	// =========================================================================
	// Configuration
	var cfg struct {
		Env string `default:"dev" envconfig:"ENV"`
		DB  struct {
			Host       string `default:"127.0.0.1:5433" envconfig:"HOST"`
			User       string `default:"postgres" envconfig:"USER"`
			Pass       string `default:"postgres" envconfig:"PASS" json:"-"` // don't print
			Database   string `default:"shared" envconfig:"DATABASE"`
			Driver     string `default:"postgres" envconfig:"DRIVER"`
			Timezone   string `default:"utc" envconfig:"TIMEZONE"`
			DisableTLS bool   `default:"true" envconfig:"DISABLE_TLS"`
		}
		Project struct {
			Name string `default:"saas-starter-kit" envconfig:"Name"`
		}
		Aws struct {
			AccessKeyID     string `envconfig:"AWS_ACCESS_KEY_ID"`              // WEB_API_AWS_AWS_ACCESS_KEY_ID or AWS_ACCESS_KEY_ID
			SecretAccessKey string `envconfig:"AWS_SECRET_ACCESS_KEY" json:"-"` // don't print
			Region          string `default:"us-east-1" envconfig:"AWS_REGION"`

			// Get an AWS session from an implicit source if no explicit
			// configuration is provided. This is useful for taking advantage of
			// EC2/ECS instance roles.
			UseRole bool `envconfig:"AWS_USE_ROLE"`
		}
	}

	// For additional details refer to https://github.com/kelseyhightower/envconfig
	if err := envconfig.Process(service, &cfg); err != nil {
		log.Fatalf("main : Parsing Config : %v", err)
	}

	if err := flag.Process(&cfg); err != nil {
		if err != flag.ErrHelp {
			log.Fatalf("main : Parsing Command Line : %v", err)
		}
		return // We displayed help.
	}

	/*
		//
		DBInstanceIdentifier:      aws.String(req.ProjectName + "-" + req.Env),


		Secret ID used to store the DB username and password across deploys.
				dbSecretId := filepath.Join(req.ProjectName, req.Env, *req.DBInstance.DBInstanceIdentifier)

	*/

	// =========================================================================
	// Log App Info

	// Print the build version for our logs. Also expose it under /debug/vars.
	expvar.NewString("build").Set(build)
	log.Printf("main : Started : Application Initializing version %q", build)
	defer log.Println("main : Completed")

	// Print the config for our logs. It's important to any credentials in the config
	// that could expose a security risk are excluded from being json encoded by
	// applying the tag `json:"-"` to the struct var.
	{
		cfgJSON, err := json.MarshalIndent(cfg, "", "    ")
		if err != nil {
			log.Fatalf("main : Marshalling Config to JSON : %v", err)
		}
		log.Printf("main : Config : %v\n", string(cfgJSON))
	}

	// =========================================================================
	// Start Database
	var dbUrl url.URL
	{
		// Query parameters.
		var q url.Values = make(map[string][]string)

		// Handle SSL Mode
		if cfg.DB.DisableTLS {
			q.Set("sslmode", "disable")
		} else {
			q.Set("sslmode", "require")
		}

		q.Set("timezone", cfg.DB.Timezone)

		// Construct url.
		dbUrl = url.URL{
			Scheme:   cfg.DB.Driver,
			User:     url.UserPassword(cfg.DB.User, cfg.DB.Pass),
			Host:     cfg.DB.Host,
			Path:     cfg.DB.Database,
			RawQuery: q.Encode(),
		}
	}

	// Register informs the sqlxtrace package of the driver that we will be using in our program.
	// It uses a default service name, in the below case "postgres.db". To use a custom service
	// name use RegisterWithServiceName.
	sqltrace.Register(cfg.DB.Driver, &pq.Driver{}, sqltrace.WithServiceName(service))
	masterDb, err := sqlxtrace.Open(cfg.DB.Driver, dbUrl.String())
	if err != nil {
		log.Fatalf("main : Register DB : %s : %v", cfg.DB.Driver, err)
	}
	defer masterDb.Close()

	// =========================================================================
	// Start Migrations

	// Execute the migrations
	if err = schema.Migrate(masterDb, log); err != nil {
		log.Fatalf("main : Migrate : %v", err)
	}
	log.Printf("main : Migrate : Completed")
}
