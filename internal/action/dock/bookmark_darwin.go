package dock

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation
#import <Foundation/Foundation.h>
#include <stdlib.h>

typedef struct {
	const void *data;
	int length;
} BookmarkResult;

BookmarkResult createBookmark(const char *path) {
	BookmarkResult r = {NULL, 0};
	@autoreleasepool {
		NSString *pathStr = [NSString stringWithUTF8String:path];
		NSURL *url = [NSURL fileURLWithPath:pathStr];
		NSError *error = nil;
		NSData *bm = [url bookmarkDataWithOptions:0
			includingResourceValuesForKeys:nil
			relativeToURL:nil
			error:&error];
		if (bm) {
			r.length = (int)[bm length];
			r.data = malloc(r.length);
			memcpy((void*)r.data, [bm bytes], r.length);
		}
	}
	return r;
}

const char *bundleID(const char *appPath) {
	@autoreleasepool {
		NSString *path = [NSString stringWithUTF8String:appPath];
		NSBundle *bundle = [NSBundle bundleWithPath:path];
		if (bundle) {
			NSString *bid = [bundle bundleIdentifier];
			if (bid) {
				return strdup([bid UTF8String]);
			}
		}
	}
	return NULL;
}

void freeCString(char *s) { free(s); }
void freeBookmarkData(void *p) { free(p); }
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// CreateBookmark generates macOS security-scoped bookmark data for a path.
// This is the binary blob stored in the "book" field of dock plist entries,
// which the Dock uses to locate and render app icons.
func CreateBookmark(path string) ([]byte, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	result := C.createBookmark(cpath)
	if result.data == nil {
		return nil, fmt.Errorf("create bookmark for %s: Foundation returned nil", path)
	}
	defer C.freeBookmarkData(unsafe.Pointer(result.data))

	return C.GoBytes(result.data, result.length), nil
}

// BundleIdentifier reads the CFBundleIdentifier from an application bundle.
// Returns empty string if the bundle or identifier cannot be read.
func BundleIdentifier(appPath string) string {
	cpath := C.CString(appPath)
	defer C.free(unsafe.Pointer(cpath))

	bid := C.bundleID(cpath)
	if bid == nil {
		return ""
	}
	defer C.freeCString(bid)

	return C.GoString(bid)
}
