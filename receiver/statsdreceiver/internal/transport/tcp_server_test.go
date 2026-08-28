// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleTCPConn_ChunkEndingOnNewlineClearsRemainder(t *testing.T) {
	// The framing loop carries a partial line across reads in `remainder`. A read
	// that ends exactly on a newline leaves nothing partial behind, so `remainder`
	// has to be cleared; otherwise the stale fragment is prepended to the next
	// line and the metric name comes out as "<truncated name><full name>".
	//
	// net.Pipe is unbuffered, so each Write below is consumed by exactly one Read
	// in handleTCPConn -- that is what makes the chunk boundaries deterministic.
	const name = "access_worker_elasped_time"
	chunks := []string{
		name + ":1|d\n" + "access_wo",               // ends mid-name
		"rker_elasped_time:2|d\n" + name + ":3|d\n", // ends exactly on a newline
		name + ":4|d\n",
	}

	server, client := net.Pipe()
	transferChan := make(chan Metric, 4*len(chunks))

	done := make(chan struct{})
	go func() {
		defer close(done)
		handleTCPConn(server, &MockReporter{}, transferChan)
	}()

	for _, chunk := range chunks {
		_, err := client.Write([]byte(chunk))
		require.NoError(t, err)
	}
	require.NoError(t, client.Close())
	<-done
	close(transferChan)

	var got []string
	for m := range transferChan {
		got = append(got, m.Raw)
	}

	assert.Equal(t, []string{
		name + ":1|d",
		name + ":2|d",
		name + ":3|d",
		name + ":4|d",
	}, got)
}
