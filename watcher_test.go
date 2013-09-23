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

func TestNew(t *testing.T) {
	base, rm := TempDir()
	defer rm()
	stream := New(
		[]string{base},
		NOW,
		time.Millisecond*50,
		CF_NODEFER|CF_FILEEVENTS)
	assert.True(t, stream.Chan != nil)
}

func TestStreamPaths(t *testing.T) {
	base, rm := TempDir()
	defer rm()
	stream := New(
		[]string{base},
		NOW,
		time.Millisecond*50,
		CF_NODEFER|CF_FILEEVENTS)
	path := stream.Paths()[0]
	assert.True(t, path == base)
}

func TestCreateRelativeToDevice(t *testing.T) {
	base, rm := TempDir()
	defer rm()

	fi, _ := os.Stat(base)
	dev := Device(fi.Sys().(*syscall.Stat_t).Dev)

	stream := NewRelative(
		dev,
		[]string{base},
		NOW,
		time.Millisecond*50,
		CF_NODEFER|CF_FILEEVENTS)
	assert.True(t, stream.Chan != nil)
}

func TestFlushAsync(t *testing.T) {
	base, rm := TempDir()
	defer rm()
	stream := New(
		[]string{base},
		NOW,
		time.Millisecond*50,
		CF_NODEFER|CF_FILEEVENTS)
	stream.Start()
	ioutil.WriteFile(base+"/holla", []byte{}, 777)
	time.Sleep(time.Millisecond * 50)
	event := stream.FlushAsync()
	assert.True(t, event != 0)
}

func TestFlush(t *testing.T) {
	base, rm := TempDir()
	defer rm()
	stream := New(
		[]string{base},
		NOW,
		time.Millisecond*50,
		CF_NODEFER|CF_FILEEVENTS)
	stream.Start()
	stream.Flush()
}

func TestStreamDevice(t *testing.T) {
	base, rm := TempDir()
	defer rm()

	fi, _ := os.Stat(base)
	dev := Device(fi.Sys().(*syscall.Stat_t).Dev)

	stream := NewRelative(
		dev,
		[]string{base},
		NOW,
		time.Millisecond*50,
		CF_NODEFER|CF_FILEEVENTS)

	adev := stream.Device()
	assert.True(t, dev == adev)
}

func TestStart(t *testing.T) {
	base, rm := TempDir()
	defer rm()

	stream := New(
		[]string{base},
		NOW,
		time.Millisecond*50,
		CF_NODEFER|CF_FILEEVENTS)
	ok := stream.Start()
	if ok != true {
		t.Fatal("failed to start the stream")
	}
}

func TestStop(t *testing.T) {
	base, rm := TempDir()
	defer rm()

	stream := New(
		[]string{base},
		NOW,
		time.Millisecond*50,
		CF_NODEFER|CF_FILEEVENTS)
	ok := stream.Start()
	if ok != true {
		t.Fatal("failed to start the stream")
	}
	stream.Stop()
}

func withNew(base string, action func(string)) {
	dummyfile := "dummyfile.txt"
	os.Create(filepath.Join(base, dummyfile))

	action(filepath.Join(base, dummyfile))

	os.Remove(filepath.Join(base, dummyfile))
}

func TestFileChanges(t *testing.T) {
	base, rm := TempDir()
	defer rm()

	s := New([]string{base}, NOW, time.Second/10, CF_FILEEVENTS)
	s.Start()
	defer s.Close()

	withNew(base, func(dummyfile string) {
		select {
		case <-s.Chan:
		case <-time.After(time.Second * 1):
			t.Errorf("should have got some file event, but timed out")
		}
	})
}

func TestEventFlags(t *testing.T) {
	base, rm := TempDir()
	defer rm()

	s := New([]string{base}, NOW, time.Second/10, CF_FILEEVENTS)
	s.Start()
	defer s.Close()

	withNew(base, func(dummyfile string) {
		select {
		case events := <-s.Chan:
			assert.Equals(t, len(events), 2)

			assert.True(t, events[1].Flags&EF_CREATED != 0)
		case <-time.After(time.Second * 1):
			t.Errorf("should have got some file event, but timed out")
		}
	})
}

func TestCanGetPath(t *testing.T) {
	base, rm := TempDir()
	defer rm()

	s := New([]string{base}, NOW, time.Second/10, CF_FILEEVENTS)
	s.Start()
	defer s.Close()

	withNew(base, func(dummyfile string) {
		select {
		case events := <-s.Chan:
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
	base, rm := TempDir()
	defer rm()

	s := New([]string{filepath.Join(base, "imaginaryfile")}, NOW, time.Second/10, CF_FILEEVENTS)
	s.Start()
	defer s.Close()

	withNew(base, func(dummyfile string) {
		select {
		case evs := <-s.Chan:
			t.Errorf("should have timed out, but received:%v", evs)
		case <-time.After(time.Second * 1):
		}
	})
}

func TestCanUnwatch(t *testing.T) {
	base, rm := TempDir()
	defer rm()

	s := New([]string{base}, NOW, time.Second/10, CF_FILEEVENTS)
	s.Start()
	s.Close()

	withNew(base, func(dummyfile string) {
		select {
		case evs, ok := <-s.Chan:
			if ok {
				t.Errorf("should have timed out, but received: %#v", evs)
			}
		case <-time.After(time.Second * 1):
		}
	})
}

func TestMultipleFile(t *testing.T) {
	base, rm := TempDir()
	defer rm()

	s := New([]string{base}, NOW, time.Second/10, CF_FILEEVENTS)
	s.Start()
	defer s.Close()

	files := []string{"holla", "huhu", "heeeho", "haha"}
	for _, f := range files {
		ioutil.WriteFile(filepath.Join(base, f), []byte{12, 32}, 0777)
	}

	events := []string{}
LOOP:
	for {
		select {
		case e, ok := <-s.Chan:
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
	path, _ := ioutil.TempDir("", "fsevents")
	path, _ = filepath.EvalSymlinks(path)
	return path, func() {
		os.RemoveAll(path)
	}
}
