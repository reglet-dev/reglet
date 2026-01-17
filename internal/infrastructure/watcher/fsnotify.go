package watcher

import (
	"sync"

	"github.com/fsnotify/fsnotify"
)

// FSNotifyWatcher implements the Watcher interface using fsnotify.
// It provides cross-platform file system monitoring with support for
// Linux (inotify), macOS (FSEvents/kqueue), and Windows (ReadDirectoryChangesW).
type FSNotifyWatcher struct {
	watcher *fsnotify.Watcher
	events  chan Event
	errors  chan error
	done    chan struct{}
	wg      sync.WaitGroup
}

// Option is a functional option for configuring FSNotifyWatcher.
type Option func(*FSNotifyWatcher)

// NewFSNotifyWatcher creates a new FSNotifyWatcher with the given options.
// The watcher starts a background goroutine to translate fsnotify events
// to the Watcher interface format.
func NewFSNotifyWatcher(opts ...Option) (*FSNotifyWatcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &FSNotifyWatcher{
		watcher: fsw,
		events:  make(chan Event),
		errors:  make(chan error),
		done:    make(chan struct{}),
	}

	// Apply options
	for _, opt := range opts {
		opt(w)
	}

	// Start event translation goroutine
	w.wg.Add(1)
	go w.run()

	return w, nil
}

// run translates fsnotify events to our Event type.
func (w *FSNotifyWatcher) run() {
	defer w.wg.Done()

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			translated := Event{
				Path: event.Name,
				Op:   translateOp(event.Op),
			}
			select {
			case w.events <- translated:
			case <-w.done:
				return
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			select {
			case w.errors <- err:
			case <-w.done:
				return
			}
		case <-w.done:
			return
		}
	}
}

// translateOp converts fsnotify.Op to our Operation type.
func translateOp(op fsnotify.Op) Operation {
	var result Operation
	if op.Has(fsnotify.Create) {
		result |= Create
	}
	if op.Has(fsnotify.Write) {
		result |= Write
	}
	if op.Has(fsnotify.Remove) {
		result |= Remove
	}
	if op.Has(fsnotify.Rename) {
		result |= Rename
	}
	if op.Has(fsnotify.Chmod) {
		result |= Chmod
	}
	return result
}

// Add starts watching the specified file or directory.
func (w *FSNotifyWatcher) Add(path string) error {
	return w.watcher.Add(path)
}

// Remove stops watching the specified file or directory.
func (w *FSNotifyWatcher) Remove(path string) error {
	return w.watcher.Remove(path)
}

// Events returns the channel for receiving file system events.
func (w *FSNotifyWatcher) Events() <-chan Event {
	return w.events
}

// Errors returns the channel for receiving watcher errors.
func (w *FSNotifyWatcher) Errors() <-chan error {
	return w.errors
}

// Close stops the watcher and releases all resources.
func (w *FSNotifyWatcher) Close() error {
	close(w.done)
	err := w.watcher.Close()
	w.wg.Wait()
	close(w.events)
	close(w.errors)
	return err
}
