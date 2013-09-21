#include <CoreServices/CoreServices.h>
#include "_cgo_export.h"

void
fswatch_callback(ConstFSEventStreamRef streamRef,
                 void *clientCallBackInfo,
                 size_t numEvents,
                 void *eventPaths,
                 const FSEventStreamEventFlags eventFlags[],
                 const FSEventStreamEventId eventIds[])
{
  watchDirsCallback(
      (FSEventStreamRef)streamRef,
      numEvents,
      eventPaths,
      (FSEventStreamEventFlags*)eventFlags,
      (FSEventStreamEventId*)eventIds);
}

FSEventStreamRef fswatch_stream_for_paths(CFMutableArrayRef pathsToWatch, FSEventStreamEventId since, CFTimeInterval latency, FSEventStreamCreateFlags flags) {
  return FSEventStreamCreate(
      NULL,
      fswatch_callback,
      NULL,
      pathsToWatch,
      since,
      latency,
      flags);
}