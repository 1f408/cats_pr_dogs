package main

import (
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/l4go/timer"
)

const DELAY_WAIT = 100 * time.Millisecond

type UpdateNotify struct {
	dir    string
	cb     func()
	is_ign func(p string) bool

	cb_ch    chan struct{}

	mtx     *sync.Mutex
	watcher *fsnotify.Watcher
}

func NewUpdateNotify(cb func(), is_ign func(p string) bool) (*UpdateNotify, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	un := &UpdateNotify{
		cb:       cb,
		is_ign:   is_ign,
		mtx:      &sync.Mutex{},
		cb_ch:    make(chan struct{}, 1),
		watcher:  watcher,
	}

	go un.cb_loop()
	go un.run()

	return un, nil
}

func (un *UpdateNotify) Close() {
	un.mtx.Lock()
	defer un.mtx.Unlock()

	un.watcher.Close()
}

func (un *UpdateNotify) SetDir(dir string) {
	dir = filepath.Clean(dir)

	un.mtx.Lock()
	defer un.mtx.Unlock()

	old_dir := un.dir
	if dir == old_dir {
		return
	}
	un.dir = dir

	un.watcher.Add(dir)
	if old_dir != "" {
		un.watcher.Remove(old_dir)
	}
}

func (un *UpdateNotify) get_dir() string {
	un.mtx.Lock()
	defer un.mtx.Unlock()

	return un.dir
}

func (un *UpdateNotify) cb_loop() {
	tm := timer.NewTimer()
	defer tm.Stop()

wait_loop:
	for {
		select {
		case _, ok := <-un.cb_ch:
			if !ok {
				return
			}
			tm.Start(DELAY_WAIT)
			continue wait_loop
		case <-tm.Recv():
		}

		if un.cb != nil {
			un.cb()
		}
	}
}

func (un *UpdateNotify) run() {
	defer close(un.cb_ch)

ev_loop:
	for {
		select {
		case ev, ok := <-un.watcher.Events:
			if !ok {
				return
			}
			w_dir := un.get_dir()

			dir, base := filepath.Split(ev.Name)
			dir = filepath.Clean(dir)
			if dir != w_dir {
				continue ev_loop
			}

			if un.is_ign != nil && un.is_ign(base) {
				continue ev_loop
			}

			un.cb_ch <- struct{}{}
		case err, ok := <-un.watcher.Errors:
			if !ok {
				return
			}
			log.Println("fsnotify error:", err)
		}
	}
}
