package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONProgressReporter(t *testing.T) {
	// Capture stderr using a pipe
	oldStderr := osStderr
	defer func() { osStderr = oldStderr }()

	r, w, err := os.Pipe()
	require.NoError(t, err)
	osStderr = w

	// Create reporter and emit an event
	reporter := newJSONProgressReporter(true)
	reporter.emitEvent("test_stage", "started", "Test message")

	// Close writer and read from pipe
	err = w.Close()
	require.NoError(t, err)
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	// Parse JSON - need to trim the newline
	jsonStr := bytes.TrimSpace(buf.Bytes())
	var event JSONProgressEvent
	err = json.Unmarshal(jsonStr, &event)
	require.NoError(t, err)

	// Verify fields
	assert.Equal(t, "test_stage", event.Stage)
	assert.Equal(t, "started", event.Status)
	assert.Equal(t, "Test message", event.Message)
	assert.WithinDuration(t, time.Now(), event.Timestamp, 5*time.Second)
}

func TestJSONProgressReporter_Disabled(t *testing.T) {
	// Capture stderr using a pipe
	oldStderr := osStderr
	defer func() { osStderr = oldStderr }()

	r, w, err := os.Pipe()
	require.NoError(t, err)
	osStderr = w

	// Create disabled reporter and emit an event
	reporter := newJSONProgressReporter(false)
	reporter.emitEvent("test_stage", "started", "Test message")

	// Close writer and read from pipe
	err = w.Close()
	require.NoError(t, err)
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	// Verify no output
	assert.Empty(t, buf.String())
}

func TestJSONProgressEvent_Marshal(t *testing.T) {
	event := JSONProgressEvent{
		Stage:     "manifest_generation",
		Status:    "started",
		Message:   "Generating manifest",
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	// Verify it's valid JSON
	var decoded JSONProgressEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.Stage, decoded.Stage)
	assert.Equal(t, event.Status, decoded.Status)
	assert.Equal(t, event.Message, decoded.Message)
	assert.True(t, event.Timestamp.Equal(decoded.Timestamp))
}
