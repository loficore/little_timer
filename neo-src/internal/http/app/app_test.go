package app

import (
	"testing"

	"little-timer/internal/timer"
)

func TestApp_ImplementsTimerRuntime(t *testing.T) {
	var _ timer.Runtime = (*App)(nil)
}
