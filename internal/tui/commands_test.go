package tui

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupCmd_TempDirRemoved(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "img.tar"), []byte("x"), 0o644))
	cmd := cleanupCmd(dir, true)
	_ = cmd() // execute the tea.Cmd
	_, err := os.Stat(dir)
	assert.True(t, os.IsNotExist(err), "temp work dir should be removed")
}

func TestCleanupCmd_PersistentDirPreserved(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "img.tar"), []byte("x"), 0o644))
	cmd := cleanupCmd(dir, false)
	_ = cmd()
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "persistent work dir should be preserved")
	_, err = os.Stat(filepath.Join(dir, "img.tar"))
	assert.NoError(t, err, "contents of persistent work dir should be preserved")
}

func TestCleanupCmd_EmptyDirNoop(t *testing.T) {
	cmd := cleanupCmd("", true)
	msg := cmd()
	assert.Nil(t, msg)
}

func TestSendOrCancel_ReturnsWhenContextCancelled(t *testing.T) {
	activity := make(chan tea.Msg) // unbuffered, never drained
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		sendOrCancel(ctx, activity, downloadDoneMsg{})
		close(done)
	}()
	select {
	case <-done: // good: fell through ctx.Done() instead of blocking
	case <-time.After(time.Second):
		t.Fatal("sendOrCancel blocked on a cancelled context")
	}
}

func TestSendOrCancel_DeliversWhenDrained(t *testing.T) {
	activity := make(chan tea.Msg, 1)
	ctx := context.Background()
	sendOrCancel(ctx, activity, downloadDoneMsg{})
	select {
	case got := <-activity:
		_, ok := got.(downloadDoneMsg)
		assert.True(t, ok)
	default:
		t.Fatal("message was not delivered")
	}
}
