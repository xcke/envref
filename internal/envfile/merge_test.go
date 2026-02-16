package envfile

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xcke/envref/internal/parser"
)

// TestMergeOverridePrecedence verifies that overlays correctly override
// base values and that the winning entry's metadata is preserved.
func TestMergeOverridePrecedence(t *testing.T) {
	tests := []struct {
		name     string
		base     []parser.Entry
		overlays [][]parser.Entry
		wantKeys []string
		want     map[string]parser.Entry
	}{
		{
			name: "single overlay overrides base value",
			base: []parser.Entry{
				{Key: "DB_HOST", Value: "localhost", Line: 1},
				{Key: "DB_PORT", Value: "5432", Line: 2},
			},
			overlays: [][]parser.Entry{
				{
					{Key: "DB_HOST", Value: "db.prod.internal", Line: 1},
				},
			},
			wantKeys: []string{"DB_HOST", "DB_PORT"},
			want: map[string]parser.Entry{
				"DB_HOST": {Key: "DB_HOST", Value: "db.prod.internal", Line: 1},
				"DB_PORT": {Key: "DB_PORT", Value: "5432", Line: 2},
			},
		},
		{
			name: "later overlay wins over earlier overlay",
			base: []parser.Entry{
				{Key: "MODE", Value: "development", Line: 1},
			},
			overlays: [][]parser.Entry{
				{
					{Key: "MODE", Value: "staging", Line: 1},
				},
				{
					{Key: "MODE", Value: "production", Line: 1},
				},
			},
			wantKeys: []string{"MODE"},
			want: map[string]parser.Entry{
				"MODE": {Key: "MODE", Value: "production", Line: 1},
			},
		},
		{
			name: "third overlay wins over second and first",
			base: []parser.Entry{
				{Key: "LEVEL", Value: "base", Line: 1},
			},
			overlays: [][]parser.Entry{
				{{Key: "LEVEL", Value: "first", Line: 1}},
				{{Key: "LEVEL", Value: "second", Line: 1}},
				{{Key: "LEVEL", Value: "third", Line: 1}},
			},
			wantKeys: []string{"LEVEL"},
			want: map[string]parser.Entry{
				"LEVEL": {Key: "LEVEL", Value: "third", Line: 1},
			},
		},
		{
			name: "each overlay adds different keys",
			base: []parser.Entry{
				{Key: "A", Value: "base_a", Line: 1},
			},
			overlays: [][]parser.Entry{
				{{Key: "B", Value: "from_ov1", Line: 1}},
				{{Key: "C", Value: "from_ov2", Line: 1}},
			},
			wantKeys: []string{"A", "B", "C"},
			want: map[string]parser.Entry{
				"A": {Key: "A", Value: "base_a", Line: 1},
				"B": {Key: "B", Value: "from_ov1", Line: 1},
				"C": {Key: "C", Value: "from_ov2", Line: 1},
			},
		},
		{
			name: "overlay preserves quote style of winning entry",
			base: []parser.Entry{
				{Key: "MSG", Value: "hello world", Line: 1, Quote: parser.QuoteDouble},
			},
			overlays: [][]parser.Entry{
				{
					{Key: "MSG", Value: "goodbye world", Line: 1, Quote: parser.QuoteSingle},
				},
			},
			wantKeys: []string{"MSG"},
			want: map[string]parser.Entry{
				"MSG": {Key: "MSG", Value: "goodbye world", Line: 1, Quote: parser.QuoteSingle},
			},
		},
		{
			name: "overlay preserves line number of winning entry",
			base: []parser.Entry{
				{Key: "FOO", Value: "from_base", Line: 10},
			},
			overlays: [][]parser.Entry{
				{
					{Key: "FOO", Value: "from_overlay", Line: 5},
				},
			},
			wantKeys: []string{"FOO"},
			want: map[string]parser.Entry{
				"FOO": {Key: "FOO", Value: "from_overlay", Line: 5},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := NewEnv()
			for _, e := range tt.base {
				base.Set(e)
			}

			overlays := make([]*Env, len(tt.overlays))
			for i, entries := range tt.overlays {
				overlays[i] = NewEnv()
				for _, e := range entries {
					overlays[i].Set(e)
				}
			}

			result := Merge(base, overlays...)

			assert.Equal(t, tt.wantKeys, result.Keys(), "key order mismatch")
			for key, wantEntry := range tt.want {
				got, ok := result.Get(key)
				require.True(t, ok, "key %q missing", key)
				assert.Equal(t, wantEntry.Value, got.Value, "value mismatch for %s", key)
				assert.Equal(t, wantEntry.Quote, got.Quote, "quote style mismatch for %s", key)
				assert.Equal(t, wantEntry.Line, got.Line, "line number mismatch for %s", key)
			}
		})
	}
}

// TestMergeRefDetection verifies that ref:// flags are correctly propagated
// through merges and that override direction (ref→plain, plain→ref) works.
func TestMergeRefDetection(t *testing.T) {
	t.Run("base ref preserved when no overlay", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "SECRET", Value: "ref://secrets/key", IsRef: true, Line: 1})
		base.Set(parser.Entry{Key: "PLAIN", Value: "hello", Line: 2})

		result := Merge(base)

		entry, ok := result.Get("SECRET")
		require.True(t, ok)
		assert.True(t, entry.IsRef, "ref flag should be preserved from base")
		assert.Equal(t, "ref://secrets/key", entry.Value)

		entry, ok = result.Get("PLAIN")
		require.True(t, ok)
		assert.False(t, entry.IsRef, "non-ref should remain non-ref")
	})

	t.Run("overlay ref overrides base non-ref", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "DB_PASS", Value: "plaintext_password", Line: 1})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "DB_PASS", Value: "ref://secrets/db_pass", IsRef: true, Line: 1})

		result := Merge(base, overlay)

		entry, _ := result.Get("DB_PASS")
		assert.True(t, entry.IsRef, "overlay ref should win over base non-ref")
		assert.Equal(t, "ref://secrets/db_pass", entry.Value)
	})

	t.Run("overlay non-ref overrides base ref", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "API_KEY", Value: "ref://secrets/api_key", IsRef: true, Line: 1})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "API_KEY", Value: "hardcoded_value_for_dev", Line: 1})

		result := Merge(base, overlay)

		entry, _ := result.Get("API_KEY")
		assert.False(t, entry.IsRef, "overlay non-ref should override base ref")
		assert.Equal(t, "hardcoded_value_for_dev", entry.Value)
	})

	t.Run("mixed refs across base and overlays", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "DB_HOST", Value: "localhost", Line: 1})
		base.Set(parser.Entry{Key: "DB_PASS", Value: "ref://secrets/db_pass", IsRef: true, Line: 2})
		base.Set(parser.Entry{Key: "API_KEY", Value: "ref://secrets/api_key", IsRef: true, Line: 3})
		base.Set(parser.Entry{Key: "DEBUG", Value: "false", Line: 4})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "DB_HOST", Value: "ref://secrets/db_host", IsRef: true, Line: 1}) // plain → ref
		overlay.Set(parser.Entry{Key: "API_KEY", Value: "test_key_123", Line: 2})                       // ref → plain
		overlay.Set(parser.Entry{Key: "NEW_REF", Value: "ref://keychain/new", IsRef: true, Line: 3})    // new ref

		result := Merge(base, overlay)

		// DB_HOST: was plain, now ref
		entry, _ := result.Get("DB_HOST")
		assert.True(t, entry.IsRef)
		assert.Equal(t, "ref://secrets/db_host", entry.Value)

		// DB_PASS: unchanged ref from base
		entry, _ = result.Get("DB_PASS")
		assert.True(t, entry.IsRef)
		assert.Equal(t, "ref://secrets/db_pass", entry.Value)

		// API_KEY: was ref, now plain
		entry, _ = result.Get("API_KEY")
		assert.False(t, entry.IsRef)
		assert.Equal(t, "test_key_123", entry.Value)

		// DEBUG: unchanged plain
		entry, _ = result.Get("DEBUG")
		assert.False(t, entry.IsRef)

		// NEW_REF: new ref from overlay
		entry, _ = result.Get("NEW_REF")
		assert.True(t, entry.IsRef)

		// Verify HasRefs/Refs work after merge
		assert.True(t, result.HasRefs())
		refs := result.Refs()
		assert.Len(t, refs, 3, "should have DB_HOST, DB_PASS, NEW_REF as refs")
	})

	t.Run("all refs from overlay replace all non-refs in base", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "A", Value: "plain_a", Line: 1})
		base.Set(parser.Entry{Key: "B", Value: "plain_b", Line: 2})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "A", Value: "ref://secrets/a", IsRef: true, Line: 1})
		overlay.Set(parser.Entry{Key: "B", Value: "ref://secrets/b", IsRef: true, Line: 2})

		result := Merge(base, overlay)

		assert.True(t, result.HasRefs())
		refs := result.Refs()
		assert.Len(t, refs, 2)
		assert.Equal(t, "A", refs[0].Key)
		assert.Equal(t, "B", refs[1].Key)
	})

	t.Run("no refs in either base or overlay", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "FOO", Value: "bar", Line: 1})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "BAZ", Value: "qux", Line: 1})

		result := Merge(base, overlay)
		assert.False(t, result.HasRefs())
		assert.Empty(t, result.Refs())
	})
}

// TestMergeImmutability verifies that neither base nor overlays are modified.
func TestMergeImmutability(t *testing.T) {
	t.Run("base is not modified", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "FOO", Value: "base_val", Line: 1})
		base.Set(parser.Entry{Key: "BAR", Value: "base_bar", Line: 2})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "FOO", Value: "overlay_val", Line: 1})
		overlay.Set(parser.Entry{Key: "NEW", Value: "new_val", Line: 2})

		_ = Merge(base, overlay)

		// Base should still have original values.
		assert.Equal(t, 2, base.Len(), "base length should be unchanged")
		entry, _ := base.Get("FOO")
		assert.Equal(t, "base_val", entry.Value, "base FOO should be unchanged")
		_, ok := base.Get("NEW")
		assert.False(t, ok, "base should not contain NEW key from overlay")
	})

	t.Run("overlay is not modified", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "FOO", Value: "base_val", Line: 1})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "FOO", Value: "overlay_val", Line: 1})
		overlay.Set(parser.Entry{Key: "ONLY_OV", Value: "ov", Line: 2})

		_ = Merge(base, overlay)

		// Overlay should still have original values.
		assert.Equal(t, 2, overlay.Len(), "overlay length should be unchanged")
		entry, _ := overlay.Get("FOO")
		assert.Equal(t, "overlay_val", entry.Value, "overlay FOO should be unchanged")
	})

	t.Run("multiple overlays are not modified", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "X", Value: "base", Line: 1})

		ov1 := NewEnv()
		ov1.Set(parser.Entry{Key: "X", Value: "ov1", Line: 1})
		ov1.Set(parser.Entry{Key: "Y", Value: "from_ov1", Line: 2})

		ov2 := NewEnv()
		ov2.Set(parser.Entry{Key: "X", Value: "ov2", Line: 1})
		ov2.Set(parser.Entry{Key: "Z", Value: "from_ov2", Line: 2})

		_ = Merge(base, ov1, ov2)

		// ov1 should be unchanged.
		assert.Equal(t, 2, ov1.Len())
		e, _ := ov1.Get("X")
		assert.Equal(t, "ov1", e.Value)
		_, ok := ov1.Get("Z")
		assert.False(t, ok, "ov1 should not contain Z from ov2")

		// ov2 should be unchanged.
		assert.Equal(t, 2, ov2.Len())
		e, _ = ov2.Get("X")
		assert.Equal(t, "ov2", e.Value)
		_, ok = ov2.Get("Y")
		assert.False(t, ok, "ov2 should not contain Y from ov1")
	})
}

// TestMergeOrderPreservation verifies insertion order across various merge scenarios.
func TestMergeOrderPreservation(t *testing.T) {
	t.Run("base keys first then new overlay keys appended", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "C", Value: "3", Line: 1})
		base.Set(parser.Entry{Key: "A", Value: "1", Line: 2})
		base.Set(parser.Entry{Key: "B", Value: "2", Line: 3})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "D", Value: "4", Line: 1})
		overlay.Set(parser.Entry{Key: "E", Value: "5", Line: 2})

		result := Merge(base, overlay)
		assert.Equal(t, []string{"C", "A", "B", "D", "E"}, result.Keys())
	})

	t.Run("overridden key stays in base position", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "FIRST", Value: "1", Line: 1})
		base.Set(parser.Entry{Key: "SECOND", Value: "2", Line: 2})
		base.Set(parser.Entry{Key: "THIRD", Value: "3", Line: 3})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "SECOND", Value: "updated", Line: 1})

		result := Merge(base, overlay)
		assert.Equal(t, []string{"FIRST", "SECOND", "THIRD"}, result.Keys())
		entry, _ := result.Get("SECOND")
		assert.Equal(t, "updated", entry.Value)
	})

	t.Run("multiple overlays: new keys appended in overlay order", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "BASE", Value: "base", Line: 1})

		ov1 := NewEnv()
		ov1.Set(parser.Entry{Key: "FROM_OV1", Value: "ov1", Line: 1})

		ov2 := NewEnv()
		ov2.Set(parser.Entry{Key: "FROM_OV2", Value: "ov2", Line: 1})

		ov3 := NewEnv()
		ov3.Set(parser.Entry{Key: "FROM_OV3", Value: "ov3", Line: 1})

		result := Merge(base, ov1, ov2, ov3)
		assert.Equal(t, []string{"BASE", "FROM_OV1", "FROM_OV2", "FROM_OV3"}, result.Keys())
	})

	t.Run("overlay key order is preserved within overlay", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "A", Value: "base", Line: 1})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "Z", Value: "z", Line: 1})
		overlay.Set(parser.Entry{Key: "M", Value: "m", Line: 2})
		overlay.Set(parser.Entry{Key: "A_NEW", Value: "a_new", Line: 3})

		result := Merge(base, overlay)
		assert.Equal(t, []string{"A", "Z", "M", "A_NEW"}, result.Keys())
	})

	t.Run("all overlay keys override all base keys — order stays base order", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "X", Value: "base_x", Line: 1})
		base.Set(parser.Entry{Key: "Y", Value: "base_y", Line: 2})
		base.Set(parser.Entry{Key: "Z", Value: "base_z", Line: 3})

		overlay := NewEnv()
		// Overlay provides them in reverse order.
		overlay.Set(parser.Entry{Key: "Z", Value: "ov_z", Line: 1})
		overlay.Set(parser.Entry{Key: "Y", Value: "ov_y", Line: 2})
		overlay.Set(parser.Entry{Key: "X", Value: "ov_x", Line: 3})

		result := Merge(base, overlay)
		// Order should follow base (X, Y, Z), not overlay (Z, Y, X).
		assert.Equal(t, []string{"X", "Y", "Z"}, result.Keys())
		e, _ := result.Get("X")
		assert.Equal(t, "ov_x", e.Value)
		e, _ = result.Get("Y")
		assert.Equal(t, "ov_y", e.Value)
		e, _ = result.Get("Z")
		assert.Equal(t, "ov_z", e.Value)
	})
}

// TestMergeEdgeCases covers boundary conditions.
func TestMergeEdgeCases(t *testing.T) {
	t.Run("merge two empty envs", func(t *testing.T) {
		base := NewEnv()
		overlay := NewEnv()
		result := Merge(base, overlay)
		assert.Equal(t, 0, result.Len())
		assert.Empty(t, result.Keys())
	})

	t.Run("merge empty base with non-empty overlay", func(t *testing.T) {
		base := NewEnv()
		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "FOO", Value: "bar", Line: 1})

		result := Merge(base, overlay)
		assert.Equal(t, 1, result.Len())
		entry, ok := result.Get("FOO")
		require.True(t, ok)
		assert.Equal(t, "bar", entry.Value)
	})

	t.Run("merge non-empty base with empty overlay", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "FOO", Value: "bar", Line: 1})
		overlay := NewEnv()

		result := Merge(base, overlay)
		assert.Equal(t, 1, result.Len())
		entry, _ := result.Get("FOO")
		assert.Equal(t, "bar", entry.Value)
	})

	t.Run("merge with no overlays returns copy of base", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "FOO", Value: "bar", Line: 1})

		result := Merge(base)
		assert.Equal(t, 1, result.Len())
		entry, _ := result.Get("FOO")
		assert.Equal(t, "bar", entry.Value)

		// Verify it's a copy, not the same object.
		base.Set(parser.Entry{Key: "FOO", Value: "modified", Line: 1})
		entry, _ = result.Get("FOO")
		assert.Equal(t, "bar", entry.Value, "result should be independent of base")
	})

	t.Run("overlay value is empty string", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "KEY", Value: "has_value", Line: 1})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "KEY", Value: "", Line: 1})

		result := Merge(base, overlay)
		entry, _ := result.Get("KEY")
		assert.Equal(t, "", entry.Value, "empty overlay value should override base")
	})

	t.Run("base value is empty string overridden by overlay", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "KEY", Value: "", Line: 1})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "KEY", Value: "now_has_value", Line: 1})

		result := Merge(base, overlay)
		entry, _ := result.Get("KEY")
		assert.Equal(t, "now_has_value", entry.Value)
	})

	t.Run("large merge with many keys", func(t *testing.T) {
		base := NewEnv()
		for i := 0; i < 100; i++ {
			base.Set(parser.Entry{
				Key:   fmt.Sprintf("KEY_%03d", i),
				Value: fmt.Sprintf("base_%d", i),
				Line:  i + 1,
			})
		}

		overlay := NewEnv()
		// Override even-numbered keys and add 50 new keys.
		for i := 0; i < 100; i += 2 {
			overlay.Set(parser.Entry{
				Key:   fmt.Sprintf("KEY_%03d", i),
				Value: fmt.Sprintf("overlay_%d", i),
				Line:  i/2 + 1,
			})
		}
		for i := 100; i < 150; i++ {
			overlay.Set(parser.Entry{
				Key:   fmt.Sprintf("KEY_%03d", i),
				Value: fmt.Sprintf("new_%d", i),
				Line:  i + 1,
			})
		}

		result := Merge(base, overlay)
		assert.Equal(t, 150, result.Len())

		// Check even keys were overridden.
		entry, _ := result.Get("KEY_000")
		assert.Equal(t, "overlay_0", entry.Value)
		entry, _ = result.Get("KEY_050")
		assert.Equal(t, "overlay_50", entry.Value)

		// Check odd keys were preserved.
		entry, _ = result.Get("KEY_001")
		assert.Equal(t, "base_1", entry.Value)
		entry, _ = result.Get("KEY_099")
		assert.Equal(t, "base_99", entry.Value)

		// Check new keys were added.
		entry, ok := result.Get("KEY_149")
		require.True(t, ok)
		assert.Equal(t, "new_149", entry.Value)

		// Verify order: base keys first (0-99), then new overlay keys (100-149).
		keys := result.Keys()
		for i := 0; i < 100; i++ {
			assert.Equal(t, fmt.Sprintf("KEY_%03d", i), keys[i])
		}
		for i := 100; i < 150; i++ {
			assert.Equal(t, fmt.Sprintf("KEY_%03d", i), keys[i])
		}
	})
}

// TestMergeThreeWayProfile simulates a realistic three-way merge:
// .env (base) ← .env.development (profile) ← .env.local (personal overrides).
func TestMergeThreeWayProfile(t *testing.T) {
	base := NewEnv()
	base.Set(parser.Entry{Key: "APP_NAME", Value: "myapp", Line: 1})
	base.Set(parser.Entry{Key: "DB_HOST", Value: "localhost", Line: 2})
	base.Set(parser.Entry{Key: "DB_PORT", Value: "5432", Line: 3})
	base.Set(parser.Entry{Key: "DB_PASS", Value: "ref://secrets/db_pass", IsRef: true, Line: 4})
	base.Set(parser.Entry{Key: "API_KEY", Value: "ref://secrets/api_key", IsRef: true, Line: 5})
	base.Set(parser.Entry{Key: "LOG_LEVEL", Value: "info", Line: 6})
	base.Set(parser.Entry{Key: "DEBUG", Value: "false", Line: 7})

	// Development profile: override DB host, log level, add dev flag.
	devProfile := NewEnv()
	devProfile.Set(parser.Entry{Key: "DB_HOST", Value: "dev-db.internal", Line: 1})
	devProfile.Set(parser.Entry{Key: "LOG_LEVEL", Value: "debug", Line: 2})
	devProfile.Set(parser.Entry{Key: "DEV_MODE", Value: "true", Line: 3})

	// Local overrides: override DB_PASS with plaintext for dev, enable debug.
	local := NewEnv()
	local.Set(parser.Entry{Key: "DB_PASS", Value: "localpass", Line: 1}) // ref → plain
	local.Set(parser.Entry{Key: "DEBUG", Value: "true", Line: 2})
	local.Set(parser.Entry{Key: "MY_CUSTOM", Value: "custom_val", Line: 3})

	result := Merge(base, devProfile, local)

	// Verify final values.
	tests := []struct {
		key     string
		value   string
		isRef   bool
		present bool
	}{
		{"APP_NAME", "myapp", false, true},             // untouched from base
		{"DB_HOST", "dev-db.internal", false, true},    // from dev profile
		{"DB_PORT", "5432", false, true},                // untouched from base
		{"DB_PASS", "localpass", false, true},           // local overrides ref
		{"API_KEY", "ref://secrets/api_key", true, true}, // ref untouched
		{"LOG_LEVEL", "debug", false, true},             // from dev profile
		{"DEBUG", "true", false, true},                  // from local
		{"DEV_MODE", "true", false, true},               // new from dev profile
		{"MY_CUSTOM", "custom_val", false, true},        // new from local
	}

	for _, tt := range tests {
		entry, ok := result.Get(tt.key)
		assert.Equal(t, tt.present, ok, "key %s presence", tt.key)
		if ok {
			assert.Equal(t, tt.value, entry.Value, "key %s value", tt.key)
			assert.Equal(t, tt.isRef, entry.IsRef, "key %s isRef", tt.key)
		}
	}

	// Verify order: base keys in original order, then new keys from overlays.
	wantOrder := []string{
		"APP_NAME", "DB_HOST", "DB_PORT", "DB_PASS", "API_KEY",
		"LOG_LEVEL", "DEBUG", "DEV_MODE", "MY_CUSTOM",
	}
	assert.Equal(t, wantOrder, result.Keys())

	// Verify ref counts.
	assert.True(t, result.HasRefs())
	refs := result.Refs()
	assert.Len(t, refs, 1, "only API_KEY should remain as ref")
	assert.Equal(t, "API_KEY", refs[0].Key)
}

// TestMergeAndInterpolate verifies that interpolation works correctly
// after merging, with overlay values available for interpolation.
func TestMergeAndInterpolate(t *testing.T) {
	t.Run("overlay value used in base interpolation", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "DB_HOST", Value: "localhost", Line: 1, Quote: parser.QuoteNone})
		base.Set(parser.Entry{Key: "DB_PORT", Value: "5432", Line: 2, Quote: parser.QuoteNone})
		base.Set(parser.Entry{Key: "DB_URL", Value: "postgres://${DB_HOST}:${DB_PORT}/mydb", Line: 3, Quote: parser.QuoteNone})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "DB_HOST", Value: "prod.db.internal", Line: 1, Quote: parser.QuoteNone})

		merged := Merge(base, overlay)
		Interpolate(merged)

		entry, _ := merged.Get("DB_URL")
		assert.Equal(t, "postgres://prod.db.internal:5432/mydb", entry.Value)
	})

	t.Run("new overlay variable referenced in base", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "GREETING", Value: "Hello ${USER_NAME}!", Line: 1, Quote: parser.QuoteNone})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "USER_NAME", Value: "Alice", Line: 1, Quote: parser.QuoteNone})

		merged := Merge(base, overlay)
		Interpolate(merged)

		entry, _ := merged.Get("GREETING")
		// USER_NAME is appended after GREETING in order, so it won't be available
		// when GREETING is interpolated (order-dependent resolution).
		assert.Equal(t, "Hello !", entry.Value, "USER_NAME is after GREETING in order, so not yet defined")
	})

	t.Run("overlay variable defined before base reference resolves correctly", func(t *testing.T) {
		// If the overlay adds a key that already exists in base AND the base
		// has a later key referencing it, the override value should be used.
		base := NewEnv()
		base.Set(parser.Entry{Key: "HOST", Value: "localhost", Line: 1, Quote: parser.QuoteNone})
		base.Set(parser.Entry{Key: "URL", Value: "http://${HOST}/api", Line: 2, Quote: parser.QuoteNone})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "HOST", Value: "example.com", Line: 1, Quote: parser.QuoteNone})

		merged := Merge(base, overlay)
		Interpolate(merged)

		entry, _ := merged.Get("URL")
		assert.Equal(t, "http://example.com/api", entry.Value, "should use overlay HOST value")
	})

	t.Run("single-quoted overlay value not interpolated", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "NAME", Value: "world", Line: 1, Quote: parser.QuoteNone})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "MSG", Value: "hello ${NAME}", Line: 1, Quote: parser.QuoteSingle})

		merged := Merge(base, overlay)
		Interpolate(merged)

		entry, _ := merged.Get("MSG")
		assert.Equal(t, "hello ${NAME}", entry.Value, "single-quoted values should not be interpolated")
	})

	t.Run("refs survive merge and interpolation", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "HOST", Value: "localhost", Line: 1, Quote: parser.QuoteNone})
		base.Set(parser.Entry{Key: "SECRET", Value: "ref://secrets/key", IsRef: true, Line: 2, Quote: parser.QuoteNone})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "HOST", Value: "prod.example.com", Line: 1, Quote: parser.QuoteNone})

		merged := Merge(base, overlay)
		Interpolate(merged)

		entry, _ := merged.Get("SECRET")
		assert.True(t, entry.IsRef)
		assert.Equal(t, "ref://secrets/key", entry.Value, "ref value should survive interpolation")
	})
}

// TestMergeWithFileLoad verifies merge behavior using actual file loading,
// simulating the real .env + .env.local workflow.
func TestMergeWithFileLoad(t *testing.T) {
	dir := t.TempDir()

	t.Run("realistic env and env.local merge", func(t *testing.T) {
		envContent := `# Production config
APP_NAME=myapp
DB_HOST=db.production.internal
DB_PORT=5432
DB_PASS=ref://secrets/db_pass
API_KEY=ref://secrets/api_key
DEBUG=false
LOG_LEVEL=warn
`
		localContent := `# Local dev overrides
DB_HOST=localhost
DEBUG=true
LOG_LEVEL=debug
LOCAL_ONLY=yes
`
		envPath := writeFile(t, dir, ".env.merge1", envContent)
		localPath := writeFile(t, dir, ".env.local.merge1", localContent)

		base, _, err := Load(envPath)
		require.NoError(t, err)
		local, _, err := Load(localPath)
		require.NoError(t, err)

		merged := Merge(base, local)

		assert.Equal(t, 8, merged.Len())

		// Overridden values.
		entry, _ := merged.Get("DB_HOST")
		assert.Equal(t, "localhost", entry.Value)
		entry, _ = merged.Get("DEBUG")
		assert.Equal(t, "true", entry.Value)
		entry, _ = merged.Get("LOG_LEVEL")
		assert.Equal(t, "debug", entry.Value)

		// Preserved base values.
		entry, _ = merged.Get("APP_NAME")
		assert.Equal(t, "myapp", entry.Value)
		entry, _ = merged.Get("DB_PORT")
		assert.Equal(t, "5432", entry.Value)

		// Ref values preserved.
		entry, _ = merged.Get("DB_PASS")
		assert.True(t, entry.IsRef)
		entry, _ = merged.Get("API_KEY")
		assert.True(t, entry.IsRef)

		// New key from local.
		entry, ok := merged.Get("LOCAL_ONLY")
		require.True(t, ok)
		assert.Equal(t, "yes", entry.Value)

		// Order: base keys first, then new overlay keys.
		wantOrder := []string{
			"APP_NAME", "DB_HOST", "DB_PORT", "DB_PASS",
			"API_KEY", "DEBUG", "LOG_LEVEL", "LOCAL_ONLY",
		}
		assert.Equal(t, wantOrder, merged.Keys())
	})

	t.Run("env.local overrides ref with plaintext", func(t *testing.T) {
		envContent := "SECRET=ref://secrets/my_secret\n"
		localContent := "SECRET=hardcoded_for_dev\n"

		envPath := writeFile(t, dir, ".env.ref_override", envContent)
		localPath := writeFile(t, dir, ".env.local.ref_override", localContent)

		base, _, err := Load(envPath)
		require.NoError(t, err)
		local, _, err := Load(localPath)
		require.NoError(t, err)

		merged := Merge(base, local)

		entry, _ := merged.Get("SECRET")
		assert.Equal(t, "hardcoded_for_dev", entry.Value)
		assert.False(t, entry.IsRef, "local plaintext should override ref")
	})

	t.Run("merge with missing env.local via LoadOptional", func(t *testing.T) {
		envContent := "FOO=bar\nBAZ=qux\n"
		envPath := writeFile(t, dir, ".env.optional_test", envContent)

		base, _, err := Load(envPath)
		require.NoError(t, err)
		local, _, err := LoadOptional(filepath.Join(dir, ".env.local.nonexistent"))
		require.NoError(t, err)

		merged := Merge(base, local)

		assert.Equal(t, 2, merged.Len())
		entry, _ := merged.Get("FOO")
		assert.Equal(t, "bar", entry.Value)
	})

	t.Run("merge then interpolate with file-loaded data", func(t *testing.T) {
		envContent := "DB_HOST=production.db\nDB_PORT=5432\nDB_URL=postgres://${DB_HOST}:${DB_PORT}/app\n"
		localContent := "DB_HOST=localhost\n"

		envPath := writeFile(t, dir, ".env.interp", envContent)
		localPath := writeFile(t, dir, ".env.local.interp", localContent)

		base, _, err := Load(envPath)
		require.NoError(t, err)
		local, _, err := Load(localPath)
		require.NoError(t, err)

		merged := Merge(base, local)
		Interpolate(merged)

		entry, _ := merged.Get("DB_URL")
		assert.Equal(t, "postgres://localhost:5432/app", entry.Value,
			"interpolation should use the merged (overridden) DB_HOST value")
	})
}

// TestMergeResolvedRefs verifies that ResolvedRefs() returns correct
// results after various merge operations.
func TestMergeResolvedRefs(t *testing.T) {
	t.Run("refs from both base and overlay are resolved", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "SECRET_A", Value: "ref://secrets/a", IsRef: true, Line: 1})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "SECRET_B", Value: "ref://keychain/b", IsRef: true, Line: 1})

		result := Merge(base, overlay)
		resolved := result.ResolvedRefs()

		assert.Len(t, resolved, 2)

		refA, ok := resolved["SECRET_A"]
		require.True(t, ok)
		assert.Equal(t, "secrets", refA.Backend)
		assert.Equal(t, "a", refA.Path)

		refB, ok := resolved["SECRET_B"]
		require.True(t, ok)
		assert.Equal(t, "keychain", refB.Backend)
		assert.Equal(t, "b", refB.Path)
	})

	t.Run("overridden ref updates resolved ref", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "DB_PASS", Value: "ref://secrets/db_pass", IsRef: true, Line: 1})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "DB_PASS", Value: "ref://keychain/db_pass_local", IsRef: true, Line: 1})

		result := Merge(base, overlay)
		resolved := result.ResolvedRefs()

		assert.Len(t, resolved, 1)
		ref, ok := resolved["DB_PASS"]
		require.True(t, ok)
		assert.Equal(t, "keychain", ref.Backend, "should use overlay's backend")
		assert.Equal(t, "db_pass_local", ref.Path, "should use overlay's path")
	})

	t.Run("ref overridden with non-ref is excluded from resolved refs", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "SECRET", Value: "ref://secrets/key", IsRef: true, Line: 1})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "SECRET", Value: "plaintext", Line: 1})

		result := Merge(base, overlay)
		resolved := result.ResolvedRefs()

		assert.Empty(t, resolved, "no refs should remain after override with plain value")
	})

	t.Run("nested ref paths are preserved through merge", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{
			Key: "DEEP_SECRET", Value: "ref://ssm/prod/db/password", IsRef: true, Line: 1,
		})

		result := Merge(base)
		resolved := result.ResolvedRefs()

		ref, ok := resolved["DEEP_SECRET"]
		require.True(t, ok)
		assert.Equal(t, "ssm", ref.Backend)
		assert.Equal(t, "prod/db/password", ref.Path)
	})
}
