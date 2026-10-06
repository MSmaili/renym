//go:build linux || darwin || windows

package fs

import (
	"context"
	"testing"
)

func BenchmarkPreparedMove(b *testing.B) {
	b.Run("prepare-close", func(b *testing.B) {
		req := moveFixture(b)
		ctx := context.Background()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			move, err := PrepareMove(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
			if err := move.Close(); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("check", func(b *testing.B) {
		req := moveFixture(b)
		ctx := context.Background()
		move, err := PrepareMove(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
		b.Cleanup(func() {
			if err := move.Close(); err != nil {
				b.Error(err)
			}
		})
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := move.Check(ctx); err != nil {
				b.Fatal(err)
			}
		}
	})
}
