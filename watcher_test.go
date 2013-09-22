package fsevents

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"syscall"

	"testing"
)
import "github.com/couchbaselabs/go.assert"

import (
	"os"
	"time"
)

func TestCurrent(t *testing.T) {
	id1 := Current()
	id2 := Current()
	assert.True(t, id1 == id2)
}

func TestLastEventBefore(t *testing.T) {
	base, rm := TempDir()
	defer rm()

	fi, _ := os.Stat(base)
	dev := Device(fi.Sys().(*syscall.Stat_t).Dev)
	id := LastEventBefore(dev, time.Now())
	assert.True(t, id != 0)
}

func TestCreate(t *testing.T) {
	base, rm := TempDir()
	defer rm()
	stream := Create(
		[]string{base},
		NOW,
		time.Millisecond*50,
		CF_NODEFER|CF_FILEEVENTS,
		func(s Stream, es []Event) {
			println(s)
		})
	assert.True(t, stream != 0)
}

func TestStreamPaths(t *testing.T) {
	base, rm := TempDir()
	defer rm()
	stream := Create(
		[]string{base},
		NOW,
		time.Millisecond*50,
		CF_NODEFER|CF_FILEEVENTS,
		func(s Stream, es []Event) {
			println(s)
		})
	path := stream.Paths()[0]
	assert.True(t, path == base)
}

func TestCreateRelativeToDevice(t *testing.T) {
	base, rm := TempDir()
	defer rm()

	fi, _ := os.Stat(base)
	dev := Device(fi.Sys().(*syscall.Stat_t).Dev)

	stream := CreateRelativeToDevice(
		dev,
		[]string{base},
		NOW,
		time.Millisecond*50,
		CF_NODEFER|CF_FILEEVENTS,
		func(s Stream, es []Event) {
			println(s)
		})
	assert.True(t, stream != 0)
}

func TestFlushAsync(t *testing.T) {
	base, rm := TempDir()
	defer rm()
	stream := Create(
		[]string{base},
		NOW,
		time.Millisecond*50,
		CF_NODEFER|CF_FILEEVENTS,
		func(s Stream, es []Event) {
			println(s)
		})
	stream.Start()
	println(stream)
	event := stream.FlushAsync()
	assert.True(t, event != 0)
}

func TestFlush(t *testing.T) {
	base, rm := TempDir()
	defer rm()
	stream := Create(
		[]string{base},
		NOW,
		time.Millisecond*50,
		CF_NODEFER|CF_FILEEVENTS,
		func(s Stream, es []Event) {
			println(s)
		})
	stream.Start()
	stream.Flush()
}

func TestStreamDevice(t *testing.T) {
	base, rm := TempDir()
	defer rm()

	fi, _ := os.Stat(base)
	dev := Device(fi.Sys().(*syscall.Stat_t).Dev)

	stream := Create(
		[]string{base},
		NOW,
		time.Millisecond*50,
		CF_NODEFER|CF_FILEEVENTS,
		func(s Stream, es []Event) {
			println(s)
		})

	adev := stream.Device()
	println(dev, adev)
	assert.True(t, dev == adev)
}

func TestStart(t *testing.T) {
	base, rm := TempDir()
	defer rm()

	stream := Create(
		[]string{base},
		NOW,
		time.Millisecond*50,
		CF_NODEFER|CF_FILEEVENTS,
		func(s Stream, es []Event) {
			println(s)
		})
	ok := stream.Start()
	if ok != true {
		t.Fatal("failed to start the stream")
	}
}

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

			assert.True(t, events[1].Flags&EF_CREATED != 0)
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
