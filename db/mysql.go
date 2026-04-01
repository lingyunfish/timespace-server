package db

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"timespace/config"
)

var mysqlDB *sql.DB

func InitMySQL() error {
	cfg := config.Get().MySQL
	var err error
	mysqlDB, err = sql.Open("mysql", cfg.DSN)
	if err != nil {
		return err
	}
	mysqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	mysqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	mysqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	return mysqlDB.Ping()
}

func GetMySQL() *sql.DB {
	return mysqlDB
}

func CloseMySQL() {
	if mysqlDB != nil {
		mysqlDB.Close()
	}
}
