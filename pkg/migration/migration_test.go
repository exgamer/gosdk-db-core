package migration

import (
	"testing"

	"github.com/go-gormigrate/gormigrate/v2"
)

func TestGormigrateOptions_ZeroValue_DefaultOptions(t *testing.T) {
	got := gormigrateOptions(Options{})

	if *got != *gormigrate.DefaultOptions {
		t.Fatalf("options = %+v, want %+v", *got, *gormigrate.DefaultOptions)
	}

	if got == gormigrate.DefaultOptions {
		t.Fatal("options must be a copy of gormigrate.DefaultOptions")
	}
}

func TestGormigrateOptions_UseTransaction(t *testing.T) {
	got := gormigrateOptions(Options{UseTransaction: true})

	want := *gormigrate.DefaultOptions
	want.UseTransaction = true

	if *got != want {
		t.Fatalf("options = %+v, want %+v", *got, want)
	}

	if gormigrate.DefaultOptions.UseTransaction {
		t.Fatal("gormigrate.DefaultOptions must not change")
	}
}

func TestRunAndRollback_EmptyList_NoError(t *testing.T) {
	cases := []struct {
		name string
		run  func() error
	}{
		{name: "Run", run: func() error { return Run(nil, nil) }},
		{name: "RunWithOptions", run: func() error { return RunWithOptions(nil, nil, Options{UseTransaction: true}) }},
		{name: "RollbackLast", run: func() error { return RollbackLast(nil, nil) }},
		{name: "RollbackLastWithOptions", run: func() error { return RollbackLastWithOptions(nil, nil, Options{UseTransaction: true}) }},
		{name: "RollbackTo", run: func() error { return RollbackTo(nil, nil, "x") }},
		{name: "RollbackToWithOptions", run: func() error { return RollbackToWithOptions(nil, nil, "x", Options{UseTransaction: true}) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
		})
	}
}
