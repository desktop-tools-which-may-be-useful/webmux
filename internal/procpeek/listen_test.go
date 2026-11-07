package procpeek

import (
	"slices"
	"testing"
)

func TestListeningPortsDedupeSortAndExclude(t *testing.T) {
	found := []ListeningPort{
		{Port: 8080, Process: "node"},
		{Port: 7681, Process: "webmux"},
		{Port: 3000, Process: "python3"},
		// The IPv6 socket of the same port, held by a child.
		{Port: 8080, Process: "worker"},
	}
	got := FilterListeningPorts(found, []int{7681})
	want := []ListeningPort{{Port: 3000, Process: "python3"}, {Port: 8080, Process: "node"}}
	if !slices.Equal(got, want) {
		t.Fatalf("FilterListeningPorts() = %+v, want %+v", got, want)
	}
	if got := FilterListeningPorts(nil, nil); got != nil {
		t.Fatalf("no sockets = %+v, want nil", got)
	}
}
