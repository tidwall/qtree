package qtree

import (
	"math/rand"
	"testing"
	"time"

	"github.com/tidwall/geoindex"
)

func init() {
	seed := time.Now().UnixNano()
	println("seed:", seed)
	rand.Seed(seed)
}

func TestGeoIndex(t *testing.T) {
	t.Run("BenchVarious", func(t *testing.T) {
		geoindex.Tests.TestBenchVarious(t, &QTree{}, 1000000)
	})
	t.Run("RandomRects", func(t *testing.T) {
		geoindex.Tests.TestRandomRects(t, &QTree{}, 10000)
	})
	t.Run("RandomPoints", func(t *testing.T) {
		geoindex.Tests.TestRandomPoints(t, &QTree{}, 10000)
	})
	t.Run("ZeroPoints", func(t *testing.T) {
		geoindex.Tests.TestZeroPoints(t, &QTree{})
	})
	t.Run("CitiesSVG", func(t *testing.T) {
		geoindex.Tests.TestCitiesSVG(t, &QTree{})
	})
}

func BenchmarkRandomInsert(b *testing.B) {
	geoindex.Tests.BenchmarkRandomInsert(b, &QTree{})
}
