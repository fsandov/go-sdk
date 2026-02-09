package logs

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLoggerKeyValuePairs(t *testing.T) {
	core, obs := observer.New(zapcore.DebugLevel)
	l := &Logger{
		zap: zap.New(core),
	}

	l.Info(context.Background(), "test msg", "key1", "val1", "key2", 42)

	entries := obs.All()
	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}

	entry := entries[0]
	fieldMap := make(map[string]zapcore.Field)
	for _, f := range entry.Context {
		fieldMap[f.Key] = f
	}

	if f, ok := fieldMap["key1"]; !ok {
		t.Error("expected field 'key1'")
	} else if f.String != "val1" {
		t.Errorf("expected key1=val1, got %v", f.String)
	}

	if _, ok := fieldMap["key2"]; !ok {
		t.Error("expected field 'key2'")
	}
}

func TestLoggerZapFields(t *testing.T) {
	core, obs := observer.New(zapcore.DebugLevel)
	l := &Logger{
		zap: zap.New(core),
	}

	l.Info(context.Background(), "test msg", zap.String("field1", "val1"))

	entries := obs.All()
	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}

	entry := entries[0]
	found := false
	for _, f := range entry.Context {
		if f.Key == "field1" && f.String == "val1" {
			found = true
		}
	}
	if !found {
		t.Error("expected zap.Field 'field1' with value 'val1'")
	}
}

func TestLoggerMixedArgs(t *testing.T) {
	core, obs := observer.New(zapcore.DebugLevel)
	l := &Logger{
		zap: zap.New(core),
	}

	l.Warn(context.Background(), "mixed", zap.Int("zapField", 1), "kvKey", "kvVal")

	entries := obs.All()
	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}

	entry := entries[0]
	fieldMap := make(map[string]zapcore.Field)
	for _, f := range entry.Context {
		fieldMap[f.Key] = f
	}

	if _, ok := fieldMap["zapField"]; !ok {
		t.Error("expected zapField")
	}
	if _, ok := fieldMap["kvKey"]; !ok {
		t.Error("expected kvKey from key-value pair")
	}
}

func TestLoggerOrphanKey(t *testing.T) {
	core, obs := observer.New(zapcore.DebugLevel)
	l := &Logger{
		zap: zap.New(core),
	}

	l.Info(context.Background(), "orphan test", "lonely")

	entries := obs.All()
	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}

	entry := entries[0]
	found := false
	for _, f := range entry.Context {
		if f.Key == "orphanKey" && f.String == "lonely" {
			found = true
		}
	}
	if !found {
		t.Error("expected orphanKey field for lonely string arg")
	}
}
