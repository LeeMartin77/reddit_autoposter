package storage_test

import (
	"testing"

	"github.com/leemartin77/reddit_autoposter/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestSetup(t *testing.T) {
	strg, err := storage.NewStorage(":memory:")
	if !assert.NoError(t, err) {
		return
	}
	defer strg.Close()
	_, err = strg.Get("someid")

	if !assert.Equal(t, storage.ErrNoToken, err) {
		return
	}

	intoken := storage.Token{
		ID:     "someid",
		Token:  "sometoken",
		Expiry: "expiry",
	}
	err = strg.Insert(&intoken)
	if !assert.NoError(t, err) {
		return
	}

	rettoken, err := strg.Get("someid")
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, intoken, *rettoken)
}
