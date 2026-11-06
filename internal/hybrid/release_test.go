package hybrid

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestReleaseMarksPersistAndClearTogether(t *testing.T) {
	dir := t.TempDir()
	r := openTestRecords(t, dir)
	a, b, c := testRecord("aaaaaaaa"), testRecord("bbbbbbbb"), testRecord("cccccccc")
	a.State, b.State = StateClosed, StateInterrupted
	seedRecords(t, r, []Record{a, b, c})
	rel := Release{ID: "rel1", RepositoryID: "repo", Path: "/p", Commit: strings.Repeat("a", 40), Branch: "main", ReleasedAt: time.Now().UTC()}

	if err := r.markReleased([]string{"aaaaaaaa", "cccccccc"}, rel); !errors.Is(err, ErrConflict) {
		t.Fatalf("active record released: %v", err)
	}
	if got, _ := r.get("aaaaaaaa"); got.Released != nil {
		t.Fatal("refused release marked a record")
	}
	if err := r.markReleased([]string{"aaaaaaaa", "bbbbbbbb"}, rel); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFile(t, dir), `"released"`) {
		t.Fatal("release mark not written immediately")
	}
	got, _ := r.get("bbbbbbbb")
	if got.Released == nil || *got.Released != rel {
		t.Fatalf("release mark = %+v", got.Released)
	}
	got.Released.Path = "changed"
	if again, _ := r.get("bbbbbbbb"); again.Released.Path != "/p" {
		t.Fatal("record clone shares the release")
	}

	ids, err := r.clearReleased("rel1")
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(ids)
	if !slices.Equal(ids, []string{"aaaaaaaa", "bbbbbbbb"}) {
		t.Fatalf("cleared %v", ids)
	}
	for _, rec := range r.all() {
		if rec.Released != nil {
			t.Fatalf("record %s still released", rec.ID)
		}
	}
	if ids, err := r.clearReleased("rel1"); err != nil || len(ids) != 0 {
		t.Fatalf("second clear: %v %v", ids, err)
	}
}
