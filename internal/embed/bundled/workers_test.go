package bundled

import "testing"

func TestEmbedWorkersLeavesHeadroom(t *testing.T) {
	cases := []struct {
		cpus, texts, want int
	}{
		{12, 9000, 10}, // big pass on a 12-core: two cores stay free
		{8, 9000, 6},
		{2, 9000, 1}, // small machines never go below one worker
		{1, 9000, 1},
		{12, 3, 3}, // never more workers than work
		{12, 0, 0}, // callers guard len==0, but stay sane anyway
	}
	for _, c := range cases {
		if got := embedWorkers(c.cpus, c.texts); got != c.want {
			t.Errorf("embedWorkers(%d, %d) = %d, want %d", c.cpus, c.texts, got, c.want)
		}
	}
}
