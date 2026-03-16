package version

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	VERSION_CONST  = "0.0.0-alpha"
	VERSION_AUTHOR = ""
)

var _currentVersion *Version
var _buildTime time.Time

type Version struct {
	Major      int    `json:"major"`
	Minor      int    `json:"minor"`
	Hotfix     int    `json:"hotfix"`
	Suffix     int    `json:"suffix"`
	Prerelease string `json:"prerelease"`
	Build      string `json:"build"`
}

func init() {
	if ver, ok := BuildVersion(VERSION_CONST); !ok {
		panic(fmt.Errorf("invalid version: %v", VERSION_CONST))
	} else {
		_currentVersion = ver
	}

	_buildTime = time.Now()
}

func CurrentVersion() *Version {
	return _currentVersion
}

func Author() string {
	return VERSION_AUTHOR
}

func BuildTime() time.Time {
	return _buildTime
}

func BuildVersion(verStr string) (*Version, bool) {
	ver := &Version{}
	buildSplits := strings.Split(verStr, "+")
	if len(buildSplits) > 2 {
		return nil, false
	}

	if len(buildSplits) == 2 {
		ver.Build = buildSplits[1]
		if !validateBuild(ver.Build) {
			return nil, false
		}
	}

	idx := strings.Index(buildSplits[0], "-")
	var prereleaseSplits []string
	if idx > 0 {
		prereleaseSplits = []string{buildSplits[0][:idx], buildSplits[0][idx+1:]}
	} else {
		prereleaseSplits = []string{buildSplits[0]}
	}

	if len(prereleaseSplits) == 2 {
		ver.Prerelease = prereleaseSplits[1]
		if !validatePrerelease(prereleaseSplits[1]) {
			return nil, false
		}
	}

	coreSplits := strings.Split(prereleaseSplits[0], ".")
	if len(coreSplits) != 3 {
		return nil, false
	}

	for _, split := range coreSplits {
		if !validateNumericIdentifier(split) {
			return nil, false
		}
	}

	ver.Major, _ = strconv.Atoi(coreSplits[0])
	ver.Minor, _ = strconv.Atoi(coreSplits[1])
	ver.Hotfix, _ = strconv.Atoi(coreSplits[2])
	return ver, true
}

func (ss *Version) Compatible(ver *Version) bool {
	return ss.Major == ver.Major && ss.Minor == ver.Minor
}

func (ss *Version) GreaterThan(ver *Version) bool {
	if ss.Major > ver.Major {
		return true
	}

	if ss.Major < ver.Major {
		return false
	}

	if ss.Minor > ver.Minor {
		return true
	}

	if ss.Minor < ver.Minor {
		return false
	}

	if ss.Hotfix > ver.Hotfix {
		return true
	}

	if ss.Hotfix < ver.Hotfix {
		return false
	}

	return prereleaseGreaterThan(ss.Prerelease, ver.Prerelease)
}

func validatePrerelease(str string) bool {
	prereleaseSplits := strings.Split(str, ".")
	for _, split := range prereleaseSplits {
		if !validatePrereleaseIdentifier(split) {
			return false
		}
	}
	return true
}

func validateBuild(str string) bool {
	prereleaseSplits := strings.Split(str, ".")
	for _, split := range prereleaseSplits {
		if !validateBuildIdentifier(split) {
			return false
		}
	}
	return true
}

func validateBuildIdentifier(str string) bool {
	return validateAlphanumericIdentifier(str)
}

func validatePrereleaseIdentifier(str string) bool {
	if len(str) == 0 {
		return false
	}

	if str[0] >= '0' && str[0] <= '9' {
		return validateNumericIdentifier(str)
	}

	return validateAlphanumericIdentifier(str)
}

func validateAlphanumericIdentifier(str string) bool {
	if len(str) == 0 {
		return false
	}

	for _, b := range str {
		if b == '-' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' {
			continue
		}

		return false
	}

	return true
}

func validateNumericIdentifier(str string) bool {
	if len(str) == 0 || len(str) > 1 && str[0] == '0' {
		return false
	}

	for _, b := range str {
		if b < '0' || b > '9' {
			return false
		}
	}

	return true
}

func prereleaseGreaterThan(lhs string, rhs string) bool {
	if len(rhs) == 0 {
		return false
	}

	if len(lhs) == 0 {
		return true
	}

	lhsSplits := strings.Split(lhs, ".")
	rhsSplits := strings.Split(rhs, ".")

	for i, split := range lhsSplits {
		if i >= len(rhsSplits) {
			return true
		}

		cmpValue := identifierCompare(split, rhsSplits[i])
		if cmpValue == 0 {
			continue
		}

		if cmpValue < 0 {
			return false
		}

		return true
	}

	if len(lhsSplits) < len(rhsSplits) {
		return false
	}

	return true
}

func identifierCompare(lhs string, rhs string) int {
	nLhs, errLhs := strconv.Atoi(lhs)
	nRhs, errRhs := strconv.Atoi(rhs)
	if errLhs != nil && errRhs != nil {
		return strings.Compare(lhs, rhs)
	}

	if errLhs != nil {
		return 1
	}

	if errRhs != nil {
		return -1
	}

	return nLhs - nRhs
}

func (ss *Version) String() string {
	if len(ss.Prerelease) > 0 && len(ss.Build) > 0 {
		return fmt.Sprintf("%d.%d.%d-%s+%s", ss.Major, ss.Minor, ss.Hotfix, ss.Prerelease, ss.Build)
	}

	if len(ss.Prerelease) > 0 {
		return fmt.Sprintf("%d.%d.%d-%s", ss.Major, ss.Minor, ss.Hotfix, ss.Prerelease)
	}

	if len(ss.Build) > 0 {
		return fmt.Sprintf("%d.%d.%d+%s", ss.Major, ss.Minor, ss.Hotfix, ss.Build)
	}

	return fmt.Sprintf("%d.%d.%d", ss.Major, ss.Minor, ss.Hotfix)
}
