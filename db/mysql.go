package db

import (
	trpc "trpc.group/trpc-go/trpc-go"
	trmysql "trpc.group/trpc-go/trpc-database/mysql"
)

const mysqlServiceName = "trpc.mysql.timespace.default"

var mysqlProxy trmysql.Client

// InitMySQL 使用 tRPC-Go 的 mysql 客户端代理（RPC 方式调用数据库）
func InitMySQL() error {
	mysqlProxy = trmysql.NewClientProxy(mysqlServiceName)
	ctx := trpc.BackgroundContext()
	var result int
	err := mysqlProxy.QueryRow(ctx, []interface{}{&result}, "SELECT 1")
	return err
}

// GetMySQLProxy 获取 tRPC MySQL 客户端代理
func GetMySQLProxy() trmysql.Client {
	return mysqlProxy
}

func CloseMySQL() {
	// tRPC mysql proxy 由框架管理生命周期，无需手动关闭
}
