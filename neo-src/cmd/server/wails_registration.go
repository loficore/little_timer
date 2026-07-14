// Package main — Wails v3 service bindings registration for non-Android builds.
//
// This file ensures wails3's static analysis finds the application.NewService()
// calls even when building without the android build tag. The android-specific
// registration lives in main_android.go; this file provides the same calls
// for all other platforms.

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	httpapp "little-timer/internal/http/app"
)

var _ = application.NewService(httpapp.NewTimerService(nil))
var _ = application.NewService(httpapp.NewHabitService(nil))
var _ = application.NewService(httpapp.NewSettingsService(nil))
var _ = application.NewService(httpapp.NewBackupService(nil))
