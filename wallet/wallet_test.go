package wallet

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestGetPublicKey(t *testing.T) {
	InitWallet()
	key, err := GetPublicFromWallet()

	assert.Nil(t, err)
	assert.NotEmpty(t, key)
}