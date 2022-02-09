package spatialmath

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestR3VectorAlmostEqual(t *testing.T) {
	test.That(t, R3VectorAlmostEqual(r3.Vector{1, 2, 3}, r3.Vector{1.001, 2, 3}, 1e-4), test.ShouldBeFalse)
	test.That(t, R3VectorAlmostEqual(r3.Vector{1, 2, 3}, r3.Vector{1.001, 2.001, 3.001}, 1e-2), test.ShouldBeTrue)
}