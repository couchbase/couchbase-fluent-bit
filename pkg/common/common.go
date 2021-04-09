package common

import "github.com/fsnotify/fsnotify"

// Inspired by https://github.com/jimmidyson/configmap-reload
func IsValidEvent(event fsnotify.Event) bool {
	return event.Op&fsnotify.Create == fsnotify.Create
}
