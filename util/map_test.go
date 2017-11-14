package util

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDynamicMapWithIntValue(t *testing.T) {
	do := DynamicMap{"key": "value", "foo": 42}

	// Existing key.
	assert.Equal(t, "value", do.Get("key", "default"))

	// Default valut.
	assert.Equal(t, "default", do.Get("other-key", "default"))

	// Implicit conversion.
	assert.Equal(t, 42, do.Get("foo", 10.0))
}

func TestDynamicMapWithBoolValue(t *testing.T) {
	do := DynamicMap{"key": "value", "foo": true}

	assert.Equal(t, true, do.Get("foo", false))
}

func TestDynamicMapConvertingStringToInt(t *testing.T) {
	do := DynamicMap{"key": "value", "Expect": "404"}

	assert.Equal(t, 404, do.Get("Expect", 200))
}
