package version_test

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/codekoala/aws-roll/version"
)

func init() {
	version.Version = "vX.Y.Z"
	version.Commit = "testing"
	version.BuildDate = "today"
}

func TestString(t *testing.T) {
	assert.Equal(t, version.String(), "vX.Y.Z-testing")
}

func TestDetailed(t *testing.T) {
	assert.Equal(t, version.Detailed(), "aws-roll vX.Y.Z\nCommit:\t\ttesting\nBuild date:\ttoday\nGo:\t\t"+runtime.Version())
}
