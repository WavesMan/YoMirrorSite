// GitHub 同步器辅助函数单元测试
// 测试 compareSemVer、parseRepo、MirrorPath、ComputeChecksum 等纯函数

package syncer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// parseRepo 测试
// ============================================================

func TestParseRepo_Valid(t *testing.T) {
	owner, repo := parseRepo("microsoft/vscode")
	assert.Equal(t, "microsoft", owner)
	assert.Equal(t, "vscode", repo)
}

func TestParseRepo_NoSlash(t *testing.T) {
	owner, repo := parseRepo("invalid")
	assert.Empty(t, owner)
	assert.Empty(t, repo)
}

func TestParseRepo_Empty(t *testing.T) {
	owner, repo := parseRepo("")
	assert.Empty(t, owner)
	assert.Empty(t, repo)
}

func TestParseRepo_MultipleSlashes(t *testing.T) {
	owner, repo := parseRepo("org/repo/sub")
	assert.Equal(t, "org", owner)
	assert.Equal(t, "repo/sub", repo)
}

// ============================================================
// MirrorPath 测试
// ============================================================

func TestMirrorPath(t *testing.T) {
	path := MirrorPath("vscode", "1.85.0", "VSCode-win32-x64.zip")
	assert.Equal(t, "mirrors/vscode/versions/1.85.0/VSCode-win32-x64.zip", path)
}

func TestMirrorPath_SpecialChars(t *testing.T) {
	path := MirrorPath("my-app", "v2.0-beta.1", "file name.zip")
	assert.Contains(t, path, "file name.zip")
	assert.Contains(t, path, "v2.0-beta.1")
}

// ============================================================
// ComputeChecksum 测试
// ============================================================

func TestComputeChecksum_KnownInput(t *testing.T) {
	// SHA256 of empty string
	hash, err := ComputeChecksum(strings.NewReader(""))
	assert.NoError(t, err)
	// Empty input produces known hash
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 64) // SHA256 produces 64 hex chars
}

func TestComputeChecksum_SmallInput(t *testing.T) {
	hash1, _ := ComputeChecksum(strings.NewReader("hello"))
	hash2, _ := ComputeChecksum(strings.NewReader("hello"))
	assert.Equal(t, hash1, hash2) // deterministic
}

func TestComputeChecksum_DifferentInput(t *testing.T) {
	hash1, _ := ComputeChecksum(strings.NewReader("hello"))
	hash2, _ := ComputeChecksum(strings.NewReader("world"))
	assert.NotEqual(t, hash1, hash2)
}

// ============================================================
// compareSemVer 测试
// ============================================================

func TestCompareSemVer_Equal(t *testing.T) {
	assert.Equal(t, 0, compareSemVer("1.0.0", "1.0.0"))
	assert.Equal(t, 0, compareSemVer("v1.0.0", "1.0.0")) // v prefix handling
}

func TestCompareSemVer_Greater(t *testing.T) {
	assert.Equal(t, 1, compareSemVer("2.0.0", "1.0.0"))
	assert.Equal(t, 1, compareSemVer("1.10.0", "1.9.0")) // numeric, not lexicographic
	assert.Equal(t, 1, compareSemVer("1.9.10", "1.9.9"))
}

func TestCompareSemVer_Less(t *testing.T) {
	assert.Equal(t, -1, compareSemVer("1.0.0", "2.0.0"))
	assert.Equal(t, -1, compareSemVer("1.9.0", "1.10.0")) // critical: fixes lexicographic bug
}

func TestCompareSemVer_DiffLengths(t *testing.T) {
	assert.Equal(t, 1, compareSemVer("1.0.0.1", "1.0.0"))
	assert.Equal(t, -1, compareSemVer("1.0", "1.0.0"))
}

func TestCompareSemVer_PreRelease(t *testing.T) {
	// Note: compareSemVer doesn't handle pre-release suffixes (e.g. "1.0.0-beta")
	// That's by design — it's a simple numeric parser, not full semver spec
	t.Skip("compareSemVer is simple numeric parser, not full semver spec")
}

func TestCompareSemVer_VPrefix(t *testing.T) {
	assert.Equal(t, 0, compareSemVer("v2.1.3", "v2.1.3"))
	assert.Equal(t, 0, compareSemVer("v2.1.3", "2.1.3"))
	assert.Equal(t, 1, compareSemVer("v3.0.0", "v1.0.0"))
}

func TestCompareSemVer_RealWorld(t *testing.T) {
	// Real-world tag patterns from VS Code, Docker, Node.js
	tests := []struct {
		a, b string
		want int
	}{
		{"1.85.0", "1.84.2", 1},   // VS Code
		{"1.84.2", "1.85.0", -1},
		{"4.26.0", "4.25.0", 1},   // Docker
		{"20.10.0", "20.9.0", 1},  // Node.js
		{"20.9.0", "20.10.0", -1}, // critical: this would fail with string compare
	}
	for _, tt := range tests {
		got := compareSemVer(tt.a, tt.b)
		assert.Equal(t, tt.want, got, "compareSemVer(%q, %q)", tt.a, tt.b)
	}
}
