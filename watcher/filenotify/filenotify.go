/*
	Adapted from Hugo https://github.com/gohugoio/hugo/
*/

// Package filenotify provides a mechanism for watching file(s) for changes.
// Generally leans on fsnotify, but provides a poll-based notifier which fsnotify does not support.
// These are wrapped up in a common interface so that either can be used interchangeably in your code.
//
// This package is adapted from https://github.com/moby/moby/tree/master/pkg/filenotify, Apache-2.0 License.
// Hopefully this can be replaced with an external package sometime in the future, see https://github.com/fsnotify/fsnotify/issues/9
package filenotify

import (
	"github.com/fsnotify/fsnotify"
)

// FileWatcher is an interface for implementing file notification watchers
type FileWatcher interface {
	Events() <-chan fsnotify.Event
	Errors() <-chan error
	Add(name string) error
	Remove(name string) error
	Close() error
}

// New tries to use an fs-event watcher
func New() (FileWatcher, error) {
	return NewEventWatcher()

}

// NewEventWatcher returns an fs-event based file watcher
func NewEventWatcher() (FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &fsNotifyWatcher{watcher}, nil
}
