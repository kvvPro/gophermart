package config

import (
	"fmt"

	"github.com/caarlos0/env/v9"
	"github.com/spf13/pflag"
)

type ServerFlags struct {
	Address                string `env:"RUN_ADDRESS"`
	DBConnection           string `env:"DATABASE_URI"`
	AccrualSystemAddress   string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	ReadingAccrualInterval int    `env:"READING_ACCRUAL_INTERVAL"`
}

func Initialize() ServerFlags {
	srvFlags := new(ServerFlags)
	// try to get vars from Flags
	pflag.StringVarP(&srvFlags.Address, "addr", "a", "localhost:8080", "Net address host:port")
	pflag.StringVarP(&srvFlags.DBConnection, "databaseURI", "d", "user=postgres password=postgres host=localhost port=5432 dbname=postgres sslmode=disable", "Connection string to DB: user=<> password=<> host=<> port=<> dbname=<>")
	pflag.StringVarP(&srvFlags.AccrualSystemAddress, "accrAddr", "r", "", "Hash key to calculate hash sum")
	pflag.IntVarP(&srvFlags.ReadingAccrualInterval, "accrInterval", "i", 5, "Interval in sec to update orders info from accrual system")

	pflag.Parse()

	fmt.Println("\nFLAGS-----------")
	fmt.Printf("RUN_ADDRESS=%v", srvFlags.Address)
	fmt.Printf("DATABASE_URI=%v", srvFlags.DBConnection)
	fmt.Printf("ACCRUAL_SYSTEM_ADDRESS=%v", srvFlags.AccrualSystemAddress)
	fmt.Printf("READING_ACCRUAL_INTERVAL=%v", srvFlags.ReadingAccrualInterval)

	// try to get vars from env
	if err := env.Parse(srvFlags); err != nil {
		panic(err)
	}
	fmt.Println("ENV-----------")
	fmt.Printf("RUN_ADDRESS=%v", srvFlags.Address)
	fmt.Printf("DATABASE_URI=%v", srvFlags.DBConnection)
	fmt.Printf("ACCRUAL_SYSTEM_ADDRESS=%v", srvFlags.AccrualSystemAddress)
	fmt.Printf("READING_ACCRUAL_INTERVAL=%v", srvFlags.ReadingAccrualInterval)

	return *srvFlags
}
