//go:build !embed_ui || android

package main

import "io/fs"

func embeddedUIFS() fs.FS { return nil }
