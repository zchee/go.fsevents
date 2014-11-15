# FSEvents bindings for Go (OS X)

[![GoDoc](https://godoc.org/github.com/go-fsnotify/fsevents?status.svg)](https://godoc.org/github.com/go-fsnotify/fsevents)

[FSEvents](https://developer.apple.com/library/mac/documentation/Darwin/Reference/FSEvents_Ref/Reference/reference.html#//apple_ref/doc/uid/FSEvents.h-DontLinkElementID_33) allows an application to monitor a whole file system or portion of it. FSEvents is only available on OS X.

This code is based on **go.fsevents** by [Steven Degutis](https://github.com/sdegutis).

## Limitations

* Creates new thread for every stream
* Does not give access to the whole FSEvents API
