//go:build linux
// +build linux

package performance

import (
	"bufio"
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
)

func readVmRSSKiB() (int, error) {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, err := strconv.Atoi(fields[1])
				if err != nil {
					return 0, err
				}
				return v, nil
			}
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return 0, os.ErrNotExist
}

func TestRSSMemoryBudget_Under800MB(t *testing.T) {
	rm := newRotationManagerForPerf(t)

	for i := 0; i < 2000; i++ {
		rm.Rotate(context.Background(), nil)
	}

	rssKiB, err := readVmRSSKiB()
	if err != nil {
		t.Fatalf("read rss: %v", err)
	}

	if rssKiB > 800*1024 {
		t.Fatalf("rss too high: %d KiB", rssKiB)
	}
}
