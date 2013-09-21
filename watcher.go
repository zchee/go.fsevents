package fsevents

/*
#cgo LDFLAGS: -framework CoreServices
#include <CoreServices/CoreServices.h>
FSEventStreamRef fswatch_create(CFMutableArrayRef, FSEventStreamEventId,
	CFTimeInterval, FSEventStreamCreateFlags);
FSEventStreamRef fswatch_create_relative_to_device(dev_t , CFMutableArrayRef,
	FSEventStreamEventId , CFTimeInterval , FSEventStreamCreateFlags);
static CFMutableArrayRef fswatch_make_mutable_array() {
  return CFArrayCreateMutable(NULL, 0, &kCFTypeArrayCallBacks);
}
void
fswatch_callback(ConstFSEventStreamRef streamRef,
                 void *clientCallBackInfo,
                 size_t numEvents,
                 void *eventPaths,
                 const FSEventStreamEventFlags eventFlags[],
                 const FSEventStreamEventId eventIds[]);

*/
import "C"
import (
	"time"

	"unsafe"
)

type CreateFlags uint32

const (
	CF_NONE       CreateFlags = 0
	CF_USECFTYPES CreateFlags = 1 << (iota - 1)
	CF_NODEFER
	CF_WATCHROOT
	CF_IGNORESELF
	CF_FILEEVENTS
)

type EventFlags uint32

const (
	EF_NONE            EventFlags = 0
	EF_MUSTSCANSUBDIRS EventFlags = 1 << (iota - 1)
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

type EventID uint64
type Device C.dev_t
type Stream C.ConstFSEventStreamRef

const NOW EventID = (1 << 64) - 1

type Event struct {
	Id    EventID
	Path  string
	Flags EventFlags
}

type Callback func(Stream, []Event)

type watchingInfo struct {
	channel chan []PathEvent
	runloop C.CFRunLoopRef
}

var watchers = make(map[C.FSEventStreamRef]watchingInfo)

type PathEvent struct {
	Path  string
	Flags EventFlags
}

func Current() EventID {
	return EventID(C.FSEventsGetCurrentEventId())
}

func LastEventBefore(dev Device, ts time.Time) EventID {
	return EventID(
		C.FSEventsGetLastEventIdForDeviceBeforeTime(
			C.dev_t(dev),
			C.CFAbsoluteTime(ts.Unix())))
}

// TODO: FSEventsPurgeEventsForDeviceUpToEventId

// func (s Stream) Paths() []string {
// 	// TODO:
// 	// C.FSEventStreamCopyPathsBeingWatched
// 	return []string{}
// }

func Create(paths []string, since EventID, interval time.Duration,
	flags CreateFlags, cb Callback) Stream {

	var stream Stream
	convertForCreate(paths, since, interval, flags,
		func(ps C.CFMutableArrayRef, s C.FSEventStreamEventId,
			i C.CFTimeInterval, fs C.FSEventStreamCreateFlags) {

			stream = Stream(C.fswatch_create(
				ps,
				s,
				i,
				fs))
		})
	return stream
}

func CreateRelativeToDevice(dev Device, paths []string, since EventID, interval time.Duration,
	flags CreateFlags, cb Callback) Stream {

	cdev := C.dev_t(dev)

	var stream Stream
	convertForCreate(paths, since, interval, flags,
		func(ps C.CFMutableArrayRef, s C.FSEventStreamEventId,
			i C.CFTimeInterval, fs C.FSEventStreamCreateFlags) {

			stream = Stream(C.fswatch_create_relative_to_device(
				cdev,
				ps,
				s,
				i,
				fs))
		})
	return stream
}

func convertForCreate(paths []string, since EventID, interval time.Duration,
	flags CreateFlags,
	cb func(C.CFMutableArrayRef,
		C.FSEventStreamEventId,
		C.CFTimeInterval,
		C.FSEventStreamCreateFlags)) {

	cpaths := C.fswatch_make_mutable_array()
	defer C.CFRelease(C.CFTypeRef(cpaths))
	for _, dir := range paths {
		path := C.CString(dir)
		defer C.free(unsafe.Pointer(path))

		str := C.CFStringCreateWithCString(nil, path, C.kCFStringEncodingUTF8)
		C.CFArrayAppendValue(cpaths, unsafe.Pointer(str))
	}

	csince := C.FSEventStreamEventId(since)
	cinterval := C.CFTimeInterval(interval / time.Second)
	cflags := C.FSEventStreamCreateFlags(flags)

	cb(cpaths, csince, cinterval, cflags)
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

		stream := C.fswatch_create(pathsToWatch,
			C.FSEventStreamEventId(NOW),
			0.1,
			C.kFSEventStreamCreateFlagNoDefer|C.kFSEventStreamCreateFlagFileEvents)
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
func watchDirsCallback(stream C.FSEventStreamRef, count C.size_t, paths **C.char, flags *C.FSEventStreamEventFlags, ids *C.FSEventStreamEventId) {
	var events []PathEvent

	for i := 0; i < int(count); i++ {
		cpaths := uintptr(unsafe.Pointer(paths)) + (uintptr(i) * unsafe.Sizeof(*paths))
		cpath := *(**C.char)(unsafe.Pointer(cpaths))

		cflags := uintptr(unsafe.Pointer(flags)) + (uintptr(i) * unsafe.Sizeof(*flags))
		cflag := *(*C.FSEventStreamEventFlags)(unsafe.Pointer(cflags))

		events = append(events, PathEvent{
			Path:  C.GoString(cpath),
			Flags: EventFlags(cflag),
		})
	}

	watchers[stream].channel <- events
}
