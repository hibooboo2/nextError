package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFailingTest(t *testing.T) {
	require.True(t, false)
}
