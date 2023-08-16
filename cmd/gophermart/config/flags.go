package config

import (
	"fmt"

	"github.com/caarlos0/env/v9"
	"github.com/spf13/pflag"
)

type ServerFlags struct {
	Address              string `env:"RUN_ADDRESS"`
	DBConnection         string `env:"DATABASE_URI"`
	AccrualSystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
}

func Initialize() ServerFlags {
	srvFlags := new(ServerFlags)
	// try to get vars from Flags
	pflag.StringVarP(&srvFlags.Address, "addr", "a", "localhost:8080", "Net address host:port")
	pflag.StringVarP(&srvFlags.DBConnection, "databaseURI", "d", "user=postgres password=postgres host=localhost port=5432 dbname=postgres sslmode=disable", "Connection string to DB: user=<> password=<> host=<> port=<> dbname=<>")
	pflag.StringVarP(&srvFlags.AccrualSystemAddress, "accrAddr", "r", "", "Hash key to calculate hash sum")

	pflag.Parse()

	fmt.Println("\nFLAGS-----------")
	fmt.Printf("ADDRESS=%v", srvFlags.Address)
	fmt.Printf("DATABASE_DSN=%v", srvFlags.DBConnection)
	fmt.Printf("DATABASE_DSN=%v", srvFlags.AccrualSystemAddress)

	// try to get vars from env
	if err := env.Parse(srvFlags); err != nil {
		panic(err)
	}
	fmt.Println("ENV-----------")
	fmt.Printf("ADDRESS=%v", srvFlags.Address)
	fmt.Printf("DATABASE_DSN=%v", srvFlags.DBConnection)
	fmt.Printf("DATABASE_DSN=%v", srvFlags.AccrualSystemAddress)

	return *srvFlags
}
