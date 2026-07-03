package job104

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJobCodeFromURL(t *testing.T) {
	got := JobCodeFromURL("https://www.104.com.tw/job/abc123?jobsource=foo")
	assert.Equal(t, "abc123", got)
}
