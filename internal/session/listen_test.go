package session

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
	got := listeningPorts(found, []int{7681})
	want := []ListeningPort{{Port: 3000, Process: "python3"}, {Port: 8080, Process: "node"}}
	if !slices.Equal(got, want) {
		t.Fatalf("listeningPorts() = %+v, want %+v", got, want)
	}
	if got := listeningPorts(nil, nil); got != nil {
		t.Fatalf("no sockets = %+v, want nil", got)
	}
}

func TestListeningPortsParticipateInDisplayComparison(t *testing.T) {
	a := Info{}
	b := Info{ListeningPorts: []ListeningPort{{Port: 5173, Process: "node"}}}
	if sameDisplay(a, b) {
		t.Fatal("new listening port was invisible")
	}
	a.ListeningPorts = []ListeningPort{{Port: 5173, Process: "node"}}
	if !sameDisplay(a, b) {
		t.Fatal("equal listening ports were compared by identity")
	}
	b.ListeningPorts = []ListeningPort{{Port: 5173, Process: "bun"}}
	if sameDisplay(a, b) {
		t.Fatal("listening process change was invisible")
	}
}
