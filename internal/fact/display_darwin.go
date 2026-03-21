package fact

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation
#include <CoreGraphics/CGDirectDisplay.h>
#include <CoreGraphics/CGDisplayConfiguration.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdint.h>

typedef struct {
	uint32_t width;
	uint32_t height;
	double   hz;
} DisplayMode;

// builtInDisplayID returns the CGDirectDisplayID for the built-in display, or 0 if not found.
static CGDirectDisplayID builtInDisplayID(void) {
	CGDirectDisplayID displays[16];
	uint32_t count = 0;
	if (CGGetActiveDisplayList(16, displays, &count) != kCGErrorSuccess) {
		return 0;
	}
	for (uint32_t i = 0; i < count; i++) {
		if (CGDisplayIsBuiltin(displays[i])) {
			return displays[i];
		}
	}
	return 0;
}

// currentBuiltInMode returns the current resolution and refresh rate of the built-in display.
static DisplayMode currentBuiltInMode(void) {
	DisplayMode m = {0, 0, 0.0};
	CGDirectDisplayID did = builtInDisplayID();
	if (did == 0) {
		return m;
	}
	CGDisplayModeRef mode = CGDisplayCopyDisplayMode(did);
	if (mode == NULL) {
		return m;
	}
	m.width  = (uint32_t)CGDisplayModeGetWidth(mode);
	m.height = (uint32_t)CGDisplayModeGetHeight(mode);
	m.hz     = CGDisplayModeGetRefreshRate(mode);
	CGDisplayModeRelease(mode);
	return m;
}

// setBuiltInDisplayMode finds a matching mode and applies it permanently.
// Returns 0 on success, -1 if no built-in display, -2 if no matching mode, -3 on config error.
static int setBuiltInDisplayMode(uint32_t wantWidth, uint32_t wantHeight, double wantHZ) {
	CGDirectDisplayID did = builtInDisplayID();
	if (did == 0) {
		return -1;
	}

	// Include HiDPI/scaled modes in the enumeration.
	CFStringRef keys[1] = { kCGDisplayShowDuplicateLowResolutionModes };
	CFBooleanRef vals[1] = { kCFBooleanTrue };
	CFDictionaryRef opts = CFDictionaryCreate(NULL,
		(const void **)keys, (const void **)vals, 1,
		&kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);

	CFArrayRef modes = CGDisplayCopyAllDisplayModes(did, opts);
	CFRelease(opts);
	if (modes == NULL) {
		return -2;
	}

	CGDisplayModeRef best = NULL;
	CFIndex count = CFArrayGetCount(modes);
	for (CFIndex i = 0; i < count; i++) {
		CGDisplayModeRef mode = (CGDisplayModeRef)CFArrayGetValueAtIndex(modes, i);
		if (!CGDisplayModeIsUsableForDesktopGUI(mode)) {
			continue;
		}
		uint32_t w = (uint32_t)CGDisplayModeGetWidth(mode);
		uint32_t h = (uint32_t)CGDisplayModeGetHeight(mode);
		double hz = CGDisplayModeGetRefreshRate(mode);
		if (w == wantWidth && h == wantHeight) {
			if (wantHZ > 0 && hz != wantHZ) {
				continue;
			}
			// Prefer higher refresh rate if no specific Hz requested.
			if (best == NULL || (wantHZ == 0 && hz > CGDisplayModeGetRefreshRate(best))) {
				best = mode;
			}
		}
	}

	if (best == NULL) {
		CFRelease(modes);
		return -2;
	}

	CGDisplayConfigRef config;
	if (CGBeginDisplayConfiguration(&config) != kCGErrorSuccess) {
		CFRelease(modes);
		return -3;
	}
	CGConfigureDisplayWithDisplayMode(config, did, best, NULL);
	CGError err = CGCompleteDisplayConfiguration(config, kCGConfigurePermanently);
	CFRelease(modes);
	if (err != kCGErrorSuccess) {
		return -3;
	}
	return 0;
}
*/
import "C"

import "fmt"

// builtInDisplayMode returns the current resolution and refresh rate of the built-in display
// using CoreGraphics. Returns zero values if no built-in display is found.
func builtInDisplayMode() (width, height int, hz int) {
	m := C.currentBuiltInMode()
	return int(m.width), int(m.height), int(m.hz)
}

// setDisplayMode finds a matching display mode for the built-in display and applies it permanently.
func setDisplayMode(width, height int, hz int) error {
	rc := C.setBuiltInDisplayMode(C.uint32_t(width), C.uint32_t(height), C.double(hz))
	switch rc {
	case 0:
		return nil
	case -1:
		return fmt.Errorf("no built-in display found")
	case -2:
		desc := fmt.Sprintf("%dx%d", width, height)
		if hz > 0 {
			desc += fmt.Sprintf("@%dHz", hz)
		}
		return fmt.Errorf("no matching display mode for %s", desc)
	case -3:
		return fmt.Errorf("CoreGraphics display configuration failed")
	default:
		return fmt.Errorf("unknown display error (code %d)", rc)
	}
}
