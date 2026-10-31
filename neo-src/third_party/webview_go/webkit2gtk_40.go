//go:build (linux || openbsd || freebsd || netbsd) && !webkit41

package webview

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.0
*/
import "C"
