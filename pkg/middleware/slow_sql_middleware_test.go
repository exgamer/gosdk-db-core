package plugin

import (
	"testing"

	"github.com/getsentry/sentry-go"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/utils/tests"
)

type slowSqlProbe struct {
	ID   uint
	Hash string
}

func TestSlowSqlSentry_ContextWithoutSqlParams(t *testing.T) {
	transport := &sentry.MockTransport{}

	err := sentry.Init(sentry.ClientOptions{Dsn: "https://key@sentry.local/1", Transport: transport})
	if err != nil {
		t.Fatalf("sentry.Init: %v", err)
	}

	t.Cleanup(func() { sentry.CurrentHub().BindClient(nil) })

	db, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{Logger: logger.Discard, DryRun: true})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	err = db.Use(SlowSqlSentryMiddleware(-1, "test-service"))
	if err != nil {
		t.Fatalf("db.Use: %v", err)
	}

	var rows []slowSqlProbe
	db.Where("hash = ?", "secret-hash-value").Find(&rows)

	events := transport.Events()
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}

	sqlData := events[0].Contexts["Sql data"]

	if _, ok := sqlData["Sql Params"]; ok {
		t.Fatalf("Sql Params in Sentry context: %+v", sqlData)
	}

	for _, key := range []string{"Table", "Sql ", "Duration", "Rows affected", "Operation"} {
		if _, ok := sqlData[key]; !ok {
			t.Fatalf("key %q missing in Sentry context: %+v", key, sqlData)
		}
	}
}
