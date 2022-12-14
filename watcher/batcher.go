/*
	Adapted from Hugo https://github.com/gohugoio/hugo/
*/

// Copyright 2020 The Hugo Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package watcher

import (
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mansoor-s/aviator/watcher/filenotify"
)

// Batcher batches file watch events in a given interval.
type Batcher struct {
	filenotify.FileWatcher
	interval time.Duration
	done     chan struct{}

	Events chan []fsnotify.Event // Events are returned on this channel
}

// New creates and starts a Batcher with the given time interval.
func New(intervalBatcher time.Duration) (*Batcher, error) {
	var err error
	var watcher filenotify.FileWatcher

	watcher, err = filenotify.New()

	if err != nil {
		return nil, err
	}

	batcher := &Batcher{}
	batcher.FileWatcher = watcher
	batcher.interval = intervalBatcher
	batcher.done = make(chan struct{}, 1)
	batcher.Events = make(chan []fsnotify.Event, 1)

	if err == nil {
		go batcher.run()
	}

	return batcher, nil
}

func (b *Batcher) run() {
	tick := time.Tick(b.interval)
	evs := make([]fsnotify.Event, 0)
OuterLoop:
	for {
		select {
		case ev := <-b.FileWatcher.Events():
			evs = append(evs, ev)
		case <-tick:
			if len(evs) == 0 {
				continue
			}
			b.Events <- evs
			evs = make([]fsnotify.Event, 0)
		case <-b.done:
			break OuterLoop
		}
	}
	close(b.done)
}

// Close stops the watching of the files.
func (b *Batcher) Close() {
	b.done <- struct{}{}
	b.FileWatcher.Close()
}
