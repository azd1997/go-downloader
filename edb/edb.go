package edb

import "github.com/azd1997/ego/edatabase"

// DB 代表数据库连接实例
// 需要注意的是，由于使用的是badgerdb，同一时间只允许有一个连接实例
// 并且这个实例是线程安全的
type DB = edatabase.Database

// OpenEDB 打开数据库
func OpenEDB(dbPath string) (DB, error) {
	return edatabase.OpenDatabase("badger", dbPath)
}

// DbExists 检测数据库是否存在
func DbExists(dbPath string) bool {
	return edatabase.DbExists("badger", dbPath)
}
