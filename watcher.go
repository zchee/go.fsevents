package fsevents

/*
#cgo LDFLAGS: -framework CoreServices
#include <CoreServices/CoreServices.h>
FSEventStreamRef fswatch_stream_for_paths(CFMutableArrayRef pathsToWatch);
static CFMutableArrayRef fswatch_make_mutable_array() {
  return CFArrayCreateMutable(NULL, 0, &kCFTypeArrayCallBacks);
}

*/
import "C"
import "unsafe"

const (
	EF_NONE            uint32 = 0
	EF_MUSTSCANSUBDIRS uint32 = 1 << (iota - 1)
	EF_USERDROPPED
	EF_KERNELDROPPED
	EF_EVENTIDSWRAPPED
	EF_HISTORYDONE
	EF_ROOTCHANGED
	EF_MOUNT
	EF_UNMOUNT

	EF_CREATED
	EF_REMOVED
	EF_INODEMETAMOD
	EF_RENAMED
	EF_MODIFIED
	EF_FINDERINFOMOD
	EF_CHANGEOWNER
	EF_XATTRMOD
	EF_ISFILE
	EF_ISDIR
	EF_ISSYMLINK
)

const (
	CF_NONE       uint32 = 0
	CF_USECFTYPES uint32 = 1 << (iota - 1)
	CF_NODEFER
	CF_WATCHROOT
	CF_IGNORESELF
	CF_FILEEVENTS
)

type watchingInfo struct {
	channel chan []PathEvent
	runloop C.CFRunLoopRef
}

var watchers = make(map[C.FSEventStreamRef]watchingInfo)

type PathEvent struct {
	Path  string
	Flags uint32
}

func Unwatch(ch chan []PathEvent) {
	for stream, info := range watchers {
		if ch == info.channel {
			C.FSEventStreamStop(stream)
			C.FSEventStreamInvalidate(stream)
			C.FSEventStreamRelease(stream)
			C.CFRunLoopStop(info.runloop)
		}
	}
}

func WatchPaths(paths []string) chan []PathEvent {
	type watchSuccessData struct {
		runloop C.CFRunLoopRef
		stream  C.FSEventStreamRef
	}

	successChan := make(chan *watchSuccessData)

	go func() {
		pathsToWatch := C.fswatch_make_mutable_array()
		defer C.CFRelease(C.CFTypeRef(pathsToWatch))

		for _, dir := range paths {
			path := C.CString(dir)
			defer C.free(unsafe.Pointer(path))

			str := C.CFStringCreateWithCString(nil, path, C.kCFStringEncodingUTF8)
			C.CFArrayAppendValue(pathsToWatch, unsafe.Pointer(str))
		}

		stream := C.fswatch_stream_for_paths(pathsToWatch)
		C.FSEventStreamScheduleWithRunLoop(stream, C.CFRunLoopGetCurrent(), C.kCFRunLoopCommonModes)

		ok := C.FSEventStreamStart(stream) != 0
		if ok {
			successChan <- &watchSuccessData{
				runloop: C.CFRunLoopGetCurrent(),
				stream:  stream,
			}
			C.CFRunLoopRun()
		} else {
			successChan <- nil
		}
	}()

	watchingData := <-successChan

	if watchingData == nil {
		return nil
	}

	newChan := make(chan []PathEvent)
	watchers[watchingData.stream] = watchingInfo{
		channel: newChan,
		runloop: watchingData.runloop,
	}
	return newChan
}

//export watchDirsCallback
func watchDirsCallback(stream C.FSEventStreamRef, count C.size_t, paths **C.char, flags *C.FSEventStreamEventFlags) {
	var events []PathEvent

	for i := 0; i < int(count); i++ {
		cpaths := uintptr(unsafe.Pointer(paths)) + (uintptr(i) * unsafe.Sizeof(*paths))
		cpath := *(**C.char)(unsafe.Pointer(cpaths))

		cflags := uintptr(unsafe.Pointer(flags)) + (uintptr(i) * unsafe.Sizeof(*flags))
		cflag := *(*C.FSEventStreamEventFlags)(unsafe.Pointer(cflags))

		events = append(events, PathEvent{
			Path:  C.GoString(cpath),
			Flags: uint32(cflag),
		})
	}

	watchers[stream].channel <- events
}
