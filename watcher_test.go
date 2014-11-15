package fsevents

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"testing"
)

func TestCurrent(t *testing.T) {
	t.Parallel()
	id1 := Current()
	id2 := Current()
	if id1 != id2 {
		t.Errorf("Expected EventID %#v to equal %#v.", id1, id2)
	}
}

func TestLastEventBefore(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	fi, _ := os.Stat(base)
	dev := Device(fi.Sys().(*syscall.Stat_t).Dev)
	id := LastEventBefore(dev, time.Now())
	if id == 0 {
		t.Errorf("Expected LastEventBefore to be a non-zero, got %#v.", id)
	}
}

func TestNew(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()
	stream := New(
		0,
		SinceNow,
		time.Millisecond*50,
		NoDefer|FileEvents,
		base)
	if stream.Chan == nil {
		t.Error("Expected stream.Chan to not be nil.")
	}
}

func TestStreamPaths(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()
	stream := New(
		0,
		SinceNow,
		time.Millisecond*50,
		NoDefer|FileEvents,
		base)
	path := stream.Paths()[0]
	if path != base {
		t.Errorf("Expected %#v to equal %#v.", path, base)
	}
}

func TestCreateRelativeToDevice(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	fi, _ := os.Stat(base)
	dev := Device(fi.Sys().(*syscall.Stat_t).Dev)

	stream := New(
		dev,
		SinceNow,
		time.Millisecond*50,
		NoDefer|FileEvents,
		base)
	if stream.Chan == nil {
		t.Error("Expected stream.Chan to not be nil.")
	}
}

func TestFlushAsync(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()
	stream := New(
		0,
		SinceNow,
		time.Millisecond*50,
		NoDefer|FileEvents,
		base)
	stream.Start()
	ioutil.WriteFile(base+"/holla", []byte{}, 777)
	time.Sleep(time.Millisecond * 50)
	event := stream.FlushAsync()
	if event == 0 {
		t.Errorf("Expected FlushAsync EventID to be a non-zero, got %#v.", event)
	}
}

func TestFlush(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()
	stream := New(
		0,
		SinceNow,
		time.Millisecond*50,
		NoDefer|FileEvents,
		base)
	stream.Start()
	stream.Flush()
}

func TestStreamDevice(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	fi, _ := os.Stat(base)
	dev := Device(fi.Sys().(*syscall.Stat_t).Dev)

	stream := New(
		dev,
		SinceNow,
		time.Millisecond*50,
		NoDefer|FileEvents,
		base)

	adev := stream.Device()
	if dev != adev {
		t.Errorf("Expected %#v to equal %#v.", dev, adev)
	}
}

func TestStart(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	stream := New(0, SinceNow, time.Millisecond*50, NoDefer|FileEvents, base)
	ok := stream.Start()
	if ok != true {
		t.Fatal("failed to start the stream")
	}
}

func withNew(base string, action func(string)) {
	dummyfile := "dummyfile.txt"
	os.Create(filepath.Join(base, dummyfile))

	action(filepath.Join(base, dummyfile))

	os.Remove(filepath.Join(base, dummyfile))
}

func TestFileChanges(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	s := New(0, SinceNow, time.Second/10, FileEvents, base)
	s.Start()
	defer s.Close()

	withNew(base, func(dummyfile string) {
		select {
		case <-s.Chan:
		case <-time.After(time.Minute):
			t.Errorf("should have got some file event, but timed out")
		}
	})
}

func TestEventFlags(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	s := New(0, SinceNow, time.Second/10, FileEvents, base)
	s.Start()
	defer s.Close()

	withNew(base, func(dummyfile string) {
		select {
		case events := <-s.Chan:
			events = getEvents(base, events)
			if len(events) != 1 {
				t.Errorf("Expected 1 event, got %#v.", len(events))
			}
			if events[0].Flags&Created != Created {
				t.Errorf("Expected event to be Created, got %#v.", events[0].Flags)
			}
		case <-time.After(time.Minute):
			t.Errorf("should have got some file event, but timed out")
		}
	})
}

func TestCanGetPath(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	s := New(0, SinceNow, time.Second/10, FileEvents, base)
	s.Start()
	defer s.Close()

	withNew(base, func(dummyfile string) {
		select {
		case events := <-s.Chan:
			events = getEvents(base, events)
			if len(events) != 1 {
				t.Errorf("Expected 1 event, got %#v.", len(events))
			}
			fullpath, _ := filepath.Abs(dummyfile)
			fullpath, _ = filepath.EvalSymlinks(fullpath)
			evPath, _ := filepath.EvalSymlinks(events[0].Path)
			if evPath != fullpath {
				t.Errorf("Expected %#v to equal %#v.", evPath, fullpath)
			}
		case <-time.After(time.Minute):
			t.Errorf("timed out")
		}
	})
}

func TestOnlyWatchesSpecifiedPaths(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	s := New(0, SinceNow, time.Second/10, FileEvents,
		filepath.Join(base, "imaginaryfile"))
	s.Start()
	defer s.Close()

	withNew(base, func(dummyfile string) {
		select {
		case evs := <-s.Chan:
			t.Errorf("should have timed out, but received:%v", evs)
		case <-time.After(time.Millisecond * 200):
		}
	})
}

func TestCanUnwatch(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	s := New(0, SinceNow, time.Second/10, FileEvents, base)
	s.Start()
	s.Close()

	withNew(base, func(dummyfile string) {
		select {
		case evs, ok := <-s.Chan:
			evs = getEvents(base, evs)
			if ok && len(evs) > 0 {
				t.Errorf("should have timed out, but received: %#v", evs)
			}
		case <-time.After(time.Millisecond * 200):
		}
	})
}

func TestMultipleFile(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	s := New(
		0, SinceNow, time.Second/10, FileEvents, base)
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
		case e := <-s.Chan:
			e = getEvents(base, e)
			for _, item := range e {
				p, _ := filepath.Rel(base, item.Path)
				events = append(events, p)
			}
			if len(events) == len(files) {
				break LOOP
			}
		case <-time.After(time.Minute):
			break LOOP
		}
	}

	es := strings.Join(events, " ")
	fs := strings.Join(files, " ")
	if es != fs {
		t.Errorf("Expected events %#v to equal files %#v.", es, fs)
	}
}

func getEvents(base string, in []Event) (out []Event) {
	for _, e := range in {
		if e.Path != base {
			out = append(out, e)
		}
	}
	return
}

func TempDir() (string, func()) {
	path, _ := ioutil.TempDir("", "fsevents")
	path, _ = filepath.EvalSymlinks(path)
	return path, func() {
		os.RemoveAll(path)
	}
}

// Create 10 folders with 10 files each, all under one top-level folder,
// for a total of 111 events.
func with100Files(base string, action func(base string)) {
	for i := 0; i < 10; i++ {
		dir := filepath.Join(base, strconv.Itoa(i)+".dir")
		os.Mkdir(dir, 0755)
		for j := 0; j < 10; j++ {
			os.Create(filepath.Join(dir, "dummy"+strconv.Itoa(j)+".txt"))
		}
	}
	action(base)
}

func Test100Files(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	s := New(0, SinceNow, time.Second/10, FileEvents, base)
	s.Start()
	defer s.Close()

	count := 0
	with100Files(base, func(base string) {
		for {
			select {
			case events := <-s.Chan:
				for _, e := range events {
					count++
					_ = e
					//log.Println("a)", count, e)
					if count >= 111 {
						return
					}
				}
			case <-time.After(time.Second * 10):
				t.Errorf("should have got received 111 file events, but timed out")
				return
			}
		}
	})
}

func Test100OldFiles(t *testing.T) {
	t.Parallel()
	base, rm := TempDir()
	defer rm()

	with100Files(base, func(base string) {})

	s := New(0, SinceAll, time.Second/10, FileEvents, base)
	s.Start()
	defer s.Close()

	count := 0
	for {
		select {
		case events := <-s.Chan:
			for _, e := range events {
				count++
				_ = e
				//log.Println("a)", count, e)
				if count >= 111 {
					return
				}
			}
		case <-time.After(time.Second * 10):
			t.Errorf("should have got received 111 file events, but timed out")
			return
		}
	}
}
