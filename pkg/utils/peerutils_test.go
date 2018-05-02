package utils

import (
	"testing"

	config "github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestIsPeerAddressSame(t *testing.T) {
	resp := IsPeerAddressSame("192.167.1.1:8080", "192.167.1.1:8080")
	assert.True(t, resp)

	resp = IsPeerAddressSame("192.167.1.1:8080", "192.167.1.1:8081")
	assert.False(t, resp)
}

func TestFormRemotePeerAddress(t *testing.T) {
	peer, err := FormRemotePeerAddress("192.168.1.1:8080")
	assert.Nil(t, err)

	config.SetDefault("defaultpeerport", "80")
	peer, err = FormRemotePeerAddress("192.168.1.1")
	assert.Equal(t, peer, "192.168.1.1:80")

	peer, err = FormRemotePeerAddress(":8080")
	assert.Contains(t, err.Error(), "Invalid peer address")

}
