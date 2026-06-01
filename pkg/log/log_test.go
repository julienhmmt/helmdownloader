package log_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/julienhmmt/helmdownloader/pkg/log"
	"github.com/stretchr/testify/assert"
)

func TestLogger_Infof(t *testing.T) {
	var buf bytes.Buffer
	l := log.New(&buf, log.LevelInfo)
	l.Infof("hello %s", "world")
	assert.Contains(t, buf.String(), "INFO hello world")
}

func TestLogger_DebugfSkippedWhenLevelInfo(t *testing.T) {
	var buf bytes.Buffer
	l := log.New(&buf, log.LevelInfo)
	l.Debugf("secret %d", 42)
	assert.Empty(t, buf.String())
}

func TestLogger_DebugfEmittedWhenLevelDebug(t *testing.T) {
	var buf bytes.Buffer
	l := log.New(&buf, log.LevelDebug)
	l.Debugf("secret %d", 42)
	assert.Contains(t, buf.String(), "DEBUG secret 42")
}

func TestLogger_Errorf(t *testing.T) {
	var buf bytes.Buffer
	l := log.New(&buf, log.LevelInfo)
	l.Errorf("boom %v", assert.AnError)
	assert.Contains(t, buf.String(), "ERROR boom")
}

func TestLogger_SilentDropsEverything(t *testing.T) {
	var buf bytes.Buffer
	l := log.New(&buf, log.LevelSilent)
	l.Infof("info")
	l.Debugf("debug")
	l.Errorf("error")
	assert.Empty(t, buf.String())
}

func TestLogger_Discard(t *testing.T) {
	l := log.Discard()
	l.Infof("info")
	l.Debugf("debug")
	l.Errorf("error")
	// No panic, no output to verify.
}

func TestLogger_TimestampPrefix(t *testing.T) {
	var buf bytes.Buffer
	l := log.New(&buf, log.LevelInfo)
	l.Infof("test")
	line := buf.String()
	assert.True(t, strings.HasPrefix(line, "["))
	assert.Contains(t, line, "] INFO test")
}

func TestLevel_String(t *testing.T) {
	tests := []struct {
		level log.Level
		want  string
	}{
		{log.LevelSilent, "SILENT"},
		{log.LevelInfo, "INFO"},
		{log.LevelDebug, "DEBUG"},
		{log.Level(99), "LEVEL(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.level.String())
		})
	}
}
