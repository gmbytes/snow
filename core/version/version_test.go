package version

import (
	"github.com/stretchr/testify/assert"
	"regexp"
	"testing"
)

func TestVersion(t *testing.T) {
	var semverRegex = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
		`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?` +
		`(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

	tests := []struct {
		version string
		ok      bool
	}{
		{"1.0.0-alpha", true},
		{"1.0.0-alpha.1", true},
		{"1.0.0-alpha.beta", true},
		{"1.0.0-beta", true},
		{"1.0.0-beta.2", true},
		{"1.0.0-beta.11", true},
		{"1.0.0-rc.1", true},
		{"1.0.0", true},
		{"1.0.0.5", false},
		{"1.0.0a", false},
		{"1.0.0-alpha-x.1+123", true},
		{"1.0.0----.---.--", true},
		{"1.0.0----.---.--+-----.---.----", true},
		{"1.0.0----.0.--+-----.---.----.00", true},
		{"1.0.0----.0.--+-----.---.----.00a", true},
		{"1.0.0----.00.--+-----.---.----.00", false},
		{"0..0", false},
		{"0.*.0", false},
		{"0.0.0-a..b", false},
		{"0.0.0-a.b.c", true},
		{"0.0.0-a.*.c", false},
		{"0.0.0-a.b.c+", false},
		{"0.0.0-a.b.c+55", true},
		{"0.0.0-a.b.c+55+44", false},
	}

	for _, test := range tests {
		ver, ok := BuildVersion(test.version)
		assert.Equal(t, test.ok, ok)
		assert.Equal(t, semverRegex.MatchString(test.version), ok, "version: "+test.version)
		if ok {
			assert.Equal(t, test.version, ver.String())
		}
	}

	var ver0, ver1, ver2, ver3, ver4, ver5, ver6, ver7, ver8, ver9, ver10 *Version
	var ok bool

	ver0, ok = BuildVersion("1.0.0-alpha")
	assert.True(t, ok)
	ver1, ok = BuildVersion("1.0.0-alpha.1")
	assert.True(t, ok)
	ver2, ok = BuildVersion("1.0.0-alpha.beta")
	assert.True(t, ok)
	ver3, ok = BuildVersion("1.0.0-beta")
	assert.True(t, ok)
	ver4, ok = BuildVersion("1.0.0-beta.2")
	assert.True(t, ok)
	ver5, ok = BuildVersion("1.0.0-beta.11")
	assert.True(t, ok)
	ver6, ok = BuildVersion("1.0.0-rc.1")
	assert.True(t, ok)
	ver7, ok = BuildVersion("1.0.0")
	assert.True(t, ok)

	assert.True(t, ver7.GreaterThan(ver6))
	assert.True(t, ver6.GreaterThan(ver5))
	assert.True(t, ver5.GreaterThan(ver4))
	assert.True(t, ver4.GreaterThan(ver3))
	assert.True(t, ver3.GreaterThan(ver2))
	assert.True(t, ver2.GreaterThan(ver1))
	assert.True(t, ver1.GreaterThan(ver0))

	ver8, ok = BuildVersion("1.0.1")
	assert.True(t, ok)
	ver9, ok = BuildVersion("1.1.0")
	assert.True(t, ok)
	ver10, ok = BuildVersion("2.0.0")
	assert.True(t, ok)

	assert.True(t, ver8.Compatible(ver7))
	assert.False(t, ver9.Compatible(ver7))
	assert.False(t, ver10.Compatible(ver7))
}
