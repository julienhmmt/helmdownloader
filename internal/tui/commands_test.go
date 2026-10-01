package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteExitSummary_Bundles(t *testing.T) {
	type testCase struct {
		name    string
		bundles []sessionBundle
		want    string
	}
	var complete = "COMPLETE: a.tar.gz\n  Images: 2 included, 0 missing\n  helmdownloader verify a.tar.gz\n  tar xzf a.tar.gz && ./load.sh\n"
	var partial = "PARTIAL: b.tar.gz\n  Images: 1 included, 2 missing\n  helmdownloader verify b.tar.gz\n  tar xzf b.tar.gz && ./load.sh\n"
	var cases = []testCase{
		{name: "no bundles"},
		{name: "complete", bundles: []sessionBundle{{path: "a.tar.gz", included: 2}}, want: "Session bundles (1):\n\n" + complete},
		{name: "partial", bundles: []sessionBundle{{path: "b.tar.gz", included: 1, missing: 2}}, want: "Session bundles (1):\n\n" + partial},
		{name: "earlier partial", bundles: []sessionBundle{{path: "b.tar.gz", included: 1, missing: 2}, {path: "a.tar.gz", included: 2}}, want: "Session bundles (2):\n\n" + partial + "\n" + complete},
		{name: "chart only", bundles: []sessionBundle{{path: "crd.tar.gz"}}, want: "Session bundles (1):\n\nCOMPLETE (chart only): crd.tar.gz\n  Images: 0 included, 0 missing\n  helmdownloader verify crd.tar.gz\n  tar xzf crd.tar.gz   # extract the chart, then helm install\n"},
		{name: "zstd", bundles: []sessionBundle{{path: "a.tar.zst", included: 2}}, want: "Session bundles (1):\n\nCOMPLETE: a.tar.zst\n  Images: 2 included, 0 missing\n  helmdownloader verify a.tar.zst\n  tar --zstd -xf a.tar.zst && ./load.sh\n"},
		{name: "chart only zstd", bundles: []sessionBundle{{path: "crd.tar.zst"}}, want: "Session bundles (1):\n\nCOMPLETE (chart only): crd.tar.zst\n  Images: 0 included, 0 missing\n  helmdownloader verify crd.tar.zst\n  tar --zstd -xf crd.tar.zst   # extract the chart, then helm install\n"},
		{name: "quoted path", bundles: []sessionBundle{{path: "my out/it's $HOME.tar.zst", included: 1}}, want: "Session bundles (1):\n\nCOMPLETE: 'my out/it'\\''s $HOME.tar.zst'\n  Images: 1 included, 0 missing\n  helmdownloader verify 'my out/it'\\''s $HOME.tar.zst'\n  tar --zstd -xf 'my out/it'\\''s $HOME.tar.zst' && ./load.sh\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var output strings.Builder
			require.NoError(t, writeExitSummary(&output, tc.bundles))
			assert.Equal(t, tc.want, output.String())
			assert.NotContains(t, output.String(), "\x1b")
		})
	}
}

func TestWriteExitSummary_OutputError(t *testing.T) {
	for _, hasBundle := range []bool{true, false} {
		t.Run(fmt.Sprintf("bundles=%t", hasBundle), func(t *testing.T) {
			var output *os.File
			var err error
			output, err = os.CreateTemp(t.TempDir(), "stdout")
			require.NoError(t, err)
			require.NoError(t, output.Close())
			var bundles []sessionBundle
			if hasBundle {
				bundles = []sessionBundle{{path: "a.tar.gz", included: 1}}
			}
			err = writeExitSummary(output, bundles)
			if hasBundle {
				assert.ErrorIs(t, err, os.ErrClosed)
				assert.ErrorContains(t, err, "write exit summary")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWriteExitSummary_BundleSize(t *testing.T) {
	var path = filepath.Join(t.TempDir(), "a.tar.gz")
	require.NoError(t, os.WriteFile(path, make([]byte, 1536), 0o600))
	var output strings.Builder
	require.NoError(t, writeExitSummary(&output, []sessionBundle{{path: path, included: 1}}))
	assert.Contains(t, output.String(), "(1.5 KiB)")
}

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
