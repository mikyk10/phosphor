package store

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewSQLiteConnection(dsn string, silent bool) (*gorm.DB, error) {
	logLevel := logger.Warn
	if silent {
		logLevel = logger.Silent
	}

	gormLogger := logger.New(
		log.New(os.Stderr, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
		},
	)

	dsn = appendSQLitePragmas(dsn)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func appendSQLitePragmas(dsn string) string {
	if dsn == "" || dsn == ":memory:" {
		name := fmt.Sprintf("wispadb_%d_%d", time.Now().UnixNano(), rand.Int()) //nolint:gosec
		return fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)", name)
	}

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)"
}
