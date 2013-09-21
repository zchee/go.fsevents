package fsevents

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"testing"
)
import "github.com/couchbaselabs/go.assert"

import (
	"os"
	"time"
)

func withCreate(base string, action func(string)) {
	dummyfile := "dummyfile.txt"
	os.Create(filepath.Join(base, dummyfile))

	action(filepath.Join(base, dummyfile))

	os.Remove(filepath.Join(base, dummyfile))
}

func TestFileChanges(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	ch := WatchPaths([]string{base})

	withCreate(base, func(dummyfile string) {
		select {
		case <-ch:
		case <-time.After(time.Second * 1):
			t.Errorf("should have got some file event, but timed out")
		}
	})
}

func TestEventFlags(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	ch := WatchPaths([]string{base})

	withCreate(base, func(dummyfile string) {
		select {
		case events := <-ch:
			assert.Equals(t, len(events), 2)

			assert.True(t, events[1].Flags&FlagItemCreated != 0)
		case <-time.After(time.Second * 1):
			t.Errorf("should have got some file event, but timed out")
		}
	})
}

func TestCanGetPath(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	ch := WatchPaths([]string{base})

	withCreate(base, func(dummyfile string) {
		select {
		case events := <-ch:
			assert.Equals(t, len(events), 2)

			fullpath, _ := filepath.Abs(dummyfile)
			fullpath, _ = filepath.EvalSymlinks(fullpath)
			evPath, _ := filepath.EvalSymlinks(events[1].Path)
			assert.Equals(t, evPath, fullpath)
		case <-time.After(time.Second * 2):
			t.Errorf("timed out")
		}
	})
}

func TestOnlyWatchesSpecifiedPaths(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	ch := WatchPaths([]string{filepath.Join(base, "imaginaryfile")})

	withCreate(base, func(dummyfile string) {
		select {
		case <-ch:
			t.Errorf("should have timed out, but got some file event")
		case <-time.After(time.Second * 1):
		}
	})
}

func TestCanUnwatch(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	ch := WatchPaths([]string{base})

	Unwatch(ch)

	withCreate(base, func(dummyfile string) {
		select {
		case <-ch:
			t.Errorf("should have timed out, but got some file event")
		case <-time.After(time.Second * 1):
		}
	})
}

func TestMultipleFile(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	f := make(chan bool)
	var ch chan ([]PathEvent)
	go func() {
		ch = WatchPaths([]string{base})
		f <- true
	}()
	<-f
	// defer Unwatch(ch)

	files := []string{"holla", "huhu", "heeeho", "haha"}
	for _, f := range files {
		ioutil.WriteFile(filepath.Join(base, f), []byte{12, 32}, 0777)
	}

	events := []string{}
LOOP:
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				break LOOP
			}
			for _, item := range e[1:] {
				p, _ := filepath.Rel(base, item.Path)
				events = append(events, p)
			}
		case <-time.After(time.Second * 5):
			break LOOP
		}
	}

	assert.Equals(t, strings.Join(events, " "), strings.Join(files, " "))
}

func TempDir() (string, func()) {
	path, _ := ioutil.TempDir("", "TempDir")
	path, _ = filepath.EvalSymlinks(path)
	return path, func() {
		os.RemoveAll(path)
	}
}
