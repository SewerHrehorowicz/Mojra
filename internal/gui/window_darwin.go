//go:build darwin && cgo

package gui

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

@interface STWindow : NSWindow
@property (nonatomic, strong) NSEvent *lastLeftMouseDownEvent;
@end

@implementation STWindow
- (BOOL)canBecomeKeyWindow {
    return YES;
}
- (BOOL)canBecomeMainWindow {
    return YES;
}
- (void)sendEvent:(NSEvent *)event {
    if (event.type == NSEventTypeLeftMouseDown) {
        self.lastLeftMouseDownEvent = event;
    }
    [super sendEvent:event];
}
@end

@interface STAppDelegate : NSObject <NSApplicationDelegate, NSWindowDelegate, WKScriptMessageHandler>
@property (nonatomic, strong) STWindow *window;
@property (nonatomic, strong) WKWebView *webView;
@property (nonatomic, copy) NSString *initialURL;
@end

@implementation STAppDelegate

static NSImage *createStoryTellerAppIcon(void) {
    NSSize size = NSMakeSize(512, 512);
    NSImage *img = [[NSImage alloc] initWithSize:size];
    [img lockFocus];

    [NSGraphicsContext saveGraphicsState];
    // Use top-left origin (0,0 at top-left, Y growing downward) to match renderStoryTellerIcon 1:1
    NSAffineTransform *flip = [NSAffineTransform transform];
    [flip translateXBy:0.0 yBy:512.0];
    [flip scaleXBy:1.0 yBy:-1.0];
    [flip concat];

    [[NSColor clearColor] setFill];
    NSRectFill(NSMakeRect(0, 0, 512, 512));

    // macOS squircle base
    NSRect cardRect = NSMakeRect(40, 40, 432, 432);
    NSBezierPath *squircle = [NSBezierPath bezierPathWithRoundedRect:cardRect xRadius:96 yRadius:96];
    [[NSColor colorWithSRGBRed:0.07 green:0.07 blue:0.09 alpha:1.0] setFill];
    [squircle fill];

    [squircle setLineWidth:4.0];
    [[NSColor colorWithSRGBRed:0.28 green:0.28 blue:0.33 alpha:1.0] setStroke];
    [squircle stroke];

    // 1. Open Book Spread (two curved pages side-by-side, x: 160..352)
    NSBezierPath *bookFill = [NSBezierPath bezierPath];
    for (int x = 160; x <= 352; x += 2) {
        CGFloat ax = fabs((CGFloat)x - 256.0);
        CGFloat arch = -14.0 * sin(M_PI * ax / 96.0);
        CGPoint pt = NSMakePoint(x, 254.0 + arch);
        if (x == 160) {
            [bookFill moveToPoint:pt];
        } else {
            [bookFill lineToPoint:pt];
        }
    }
    for (int x = 352; x >= 160; x -= 2) {
        CGFloat ax = fabs((CGFloat)x - 256.0);
        CGFloat arch = -14.0 * sin(M_PI * ax / 96.0);
        [bookFill lineToPoint:NSMakePoint(x, 356.0 + arch)];
    }
    [bookFill closePath];
    [[NSColor colorWithSRGBRed:0.13 green:0.13 blue:0.16 alpha:1.0] setFill];
    [bookFill fill];
    [bookFill setLineWidth:10.0];
    [bookFill setLineJoinStyle:NSLineJoinStyleRound];
    [[NSColor colorWithSRGBRed:0.95 green:0.95 blue:0.97 alpha:1.0] setStroke];
    [bookFill stroke];

    // Stacked second page curve underneath
    NSBezierPath *bottomSheets = [NSBezierPath bezierPath];
    for (int x = 164; x <= 348; x += 2) {
        CGFloat ax = fabs((CGFloat)x - 256.0);
        CGFloat arch = -14.0 * sin(M_PI * ax / 96.0);
        CGPoint pt = NSMakePoint(x, 374.0 + arch);
        if (x == 164) {
            [bottomSheets moveToPoint:pt];
        } else {
            [bottomSheets lineToPoint:pt];
        }
    }
    [bottomSheets setLineWidth:8.0];
    [bottomSheets setLineCapStyle:NSLineCapStyleRound];
    [[NSColor colorWithSRGBRed:0.95 green:0.95 blue:0.97 alpha:1.0] setStroke];
    [bottomSheets stroke];

    // Center spine binding
    NSBezierPath *spine = [NSBezierPath bezierPath];
    [spine moveToPoint:NSMakePoint(256, 254)];
    [spine lineToPoint:NSMakePoint(256, 360)];
    [spine setLineWidth:9.0];
    [spine setLineCapStyle:NSLineCapStyleRound];
    [[NSColor colorWithSRGBRed:0.95 green:0.95 blue:0.97 alpha:1.0] setStroke];
    [spine stroke];

    // 2. Studio Headphones (top arch + cushioned earcups)
    NSBezierPath *headband = [NSBezierPath bezierPath];
    for (int i = 0; i <= 60; i++) {
        double t = M_PI * (double)i / 60.0;
        CGFloat hx = 256.0 - 138.0 * cos(t);
        CGFloat hy = 244.0 - 138.0 * sin(t);
        if (i == 0) {
            [headband moveToPoint:NSMakePoint(hx, hy)];
        } else {
            [headband lineToPoint:NSMakePoint(hx, hy)];
        }
    }
    [headband setLineWidth:14.0];
    [headband setLineCapStyle:NSLineCapStyleRound];
    [[NSColor colorWithSRGBRed:0.90 green:0.90 blue:0.93 alpha:1.0] setStroke];
    [headband stroke];

    NSBezierPath *leftCup = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(104.5, 220.5, 27.0, 89.0) xRadius:13.5 yRadius:13.5];
    NSBezierPath *rightCup = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(380.5, 220.5, 27.0, 89.0) xRadius:13.5 yRadius:13.5];
    [[NSColor colorWithSRGBRed:0.90 green:0.90 blue:0.93 alpha:1.0] setFill];
    [leftCup fill];
    [rightCup fill];

    // 3. 5-Bar Audio Equalizer above the open book
    NSBezierPath *eqBars = [NSBezierPath bezierPath];
    [eqBars moveToPoint:NSMakePoint(200, 188)];
    [eqBars lineToPoint:NSMakePoint(200, 216)];
    [eqBars moveToPoint:NSMakePoint(228, 170)];
    [eqBars lineToPoint:NSMakePoint(228, 216)];
    [eqBars moveToPoint:NSMakePoint(256, 152)];
    [eqBars lineToPoint:NSMakePoint(256, 216)];
    [eqBars moveToPoint:NSMakePoint(284, 170)];
    [eqBars lineToPoint:NSMakePoint(284, 216)];
    [eqBars moveToPoint:NSMakePoint(312, 188)];
    [eqBars lineToPoint:NSMakePoint(312, 216)];
    [eqBars setLineWidth:12.0];
    [eqBars setLineCapStyle:NSLineCapStyleRound];
    [[NSColor colorWithSRGBRed:0.76 green:0.76 blue:0.82 alpha:1.0] setStroke];
    [eqBars stroke];

    [NSGraphicsContext restoreGraphicsState];
    [img unlockFocus];
    return img;
}

- (void)hideSystemWindowButtons {
    if (self.window == nil) {
        return;
    }
    NSWindowButton buttons[] = {
        NSWindowCloseButton,
        NSWindowMiniaturizeButton,
        NSWindowZoomButton
    };
    for (int i = 0; i < 3; i++) {
        NSButton *btn = [self.window standardWindowButton:buttons[i]];
        if (btn != nil) {
            [btn setHidden:YES];
            [btn setEnabled:NO];
            NSView *titlebarView = [btn superview];
            if (titlebarView != nil) {
                [titlebarView setHidden:YES];
                NSView *titlebarContainer = [titlebarView superview];
                if (titlebarContainer != nil) {
                    [titlebarContainer setHidden:YES];
                }
            }
        }
    }
    NSView *themeFrame = [[self.window contentView] superview];
    if (themeFrame != nil) {
        for (NSView *sub in [themeFrame subviews]) {
            if (sub != [self.window contentView] && [NSStringFromClass([sub class]) containsString:@"Titlebar"]) {
                [sub setHidden:YES];
            }
        }
    }
    for (NSWindow *win in [NSApp windows]) {
        if (win != self.window && [NSStringFromClass([win class]) containsString:@"ToolbarFullScreen"]) {
            [win setIgnoresMouseEvents:YES];
            [win setOpaque:NO];
            [win setHasShadow:NO];
            [win setBackgroundColor:[NSColor clearColor]];
            [win setAlphaValue:0.0];
            if ([win contentView] != nil) {
                [[win contentView] setHidden:YES];
            }
        }
    }
}

- (void)windowDidUpdate:(NSNotification *)notification {
    [self hideSystemWindowButtons];
}

- (void)windowDidResize:(NSNotification *)notification {
    [self hideSystemWindowButtons];
}

- (void)windowDidBecomeKey:(NSNotification *)notification {
    [self hideSystemWindowButtons];
}

- (void)syncWebViewFullscreenState:(BOOL)isFullscreen {
    if (self.webView == nil) {
        return;
    }
    NSString *js = isFullscreen
        ? @"if (typeof setFullscreenUIState === 'function') setFullscreenUIState(true);"
        : @"if (typeof setFullscreenUIState === 'function') setFullscreenUIState(false);";
    [self.webView evaluateJavaScript:js completionHandler:nil];
}

- (void)windowWillEnterFullScreen:(NSNotification *)notification {
    [self hideSystemWindowButtons];
    [self syncWebViewFullscreenState:YES];
}

- (void)windowDidEnterFullScreen:(NSNotification *)notification {
    [self hideSystemWindowButtons];
    [self syncWebViewFullscreenState:YES];
    dispatch_async(dispatch_get_main_queue(), ^{
        [self hideSystemWindowButtons];
    });
}

- (void)windowWillExitFullScreen:(NSNotification *)notification {
    [self hideSystemWindowButtons];
    [self syncWebViewFullscreenState:NO];
}

- (void)windowDidExitFullScreen:(NSNotification *)notification {
    [self hideSystemWindowButtons];
    [self syncWebViewFullscreenState:NO];
    dispatch_async(dispatch_get_main_queue(), ^{
        [self hideSystemWindowButtons];
    });
}

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
    [NSApp setApplicationIconImage:createStoryTellerAppIcon()];
    NSRect visibleFrame = [[NSScreen mainScreen] visibleFrame];
    CGFloat winW = MIN(1440.0, visibleFrame.size.width * 0.92);
    CGFloat winH = MIN(900.0, visibleFrame.size.height * 0.90);
    NSRect initialFrame = NSMakeRect(
        visibleFrame.origin.x + (visibleFrame.size.width - winW) * 0.5,
        visibleFrame.origin.y + (visibleFrame.size.height - winH) * 0.5,
        winW,
        winH
    );

    NSUInteger style = NSWindowStyleMaskTitled |
                       NSWindowStyleMaskClosable |
                       NSWindowStyleMaskMiniaturizable |
                       NSWindowStyleMaskResizable |
                       NSWindowStyleMaskFullSizeContentView;

    self.window = [[STWindow alloc] initWithContentRect:initialFrame
                                              styleMask:style
                                                backing:NSBackingStoreBuffered
                                                  defer:NO];
    [self.window setTitle:@"StoryTeller"];
    [self.window setTitleVisibility:NSWindowTitleHidden];
    [self.window setTitlebarAppearsTransparent:YES];
    if ([self.window respondsToSelector:@selector(setTitlebarSeparatorStyle:)]) {
        [self.window setTitlebarSeparatorStyle:NSTitlebarSeparatorStyleNone];
    }
    [self.window setMovable:YES];
    [self.window setMovableByWindowBackground:NO];
    [self.window setBackgroundColor:[NSColor colorWithSRGBRed:0.08 green:0.07 blue:0.06 alpha:1.0]];
    [self.window setCollectionBehavior:NSWindowCollectionBehaviorFullScreenPrimary];
    [self.window setDelegate:self];
    [self hideSystemWindowButtons];

    WKWebViewConfiguration *config = [[WKWebViewConfiguration alloc] init];
    [config.preferences setValue:@YES forKey:@"fullScreenEnabled"];
    [[config userContentController] addScriptMessageHandler:self name:@"storytellerHost"];

    self.webView = [[WKWebView alloc] initWithFrame:[[self.window contentView] bounds]
                                      configuration:config];
    [self.webView setAutoresizingMask:(NSViewWidthSizable | NSViewHeightSizable)];
    [self.webView setValue:@NO forKey:@"drawsBackground"];

    [[self.window contentView] addSubview:self.webView];

    NSURL *url = [NSURL URLWithString:self.initialURL];
    NSURLRequest *req = [NSURLRequest requestWithURL:url];
    [self.webView loadRequest:req];

    [self.window makeKeyAndOrderFront:nil];
    [self hideSystemWindowButtons];
    [NSApp activateIgnoringOtherApps:YES];
}

- (void)userContentController:(WKUserContentController *)userContentController
      didReceiveScriptMessage:(WKScriptMessage *)message {
    if (![message.body isKindOfClass:[NSString class]]) {
        return;
    }
    NSString *cmd = (NSString *)message.body;
    if ([cmd isEqualToString:@"quit"]) {
        [NSApp terminate:nil];
    } else if ([cmd isEqualToString:@"toggle-fullscreen"]) {
        [self.window toggleFullScreen:nil];
    } else if ([cmd isEqualToString:@"start-window-drag"]) {
        NSEvent *dragEvent = self.window.lastLeftMouseDownEvent ?: [NSApp currentEvent];
        if (dragEvent != nil && self.window != nil) {
            [self.window performWindowDragWithEvent:dragEvent];
        }
    }
}

- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication *)sender {
    return YES;
}

@end

static STAppDelegate *gDelegate = nil;
static BOOL gNativeLoopRunning = NO;

static void runNativeMacOSWebView(const char *urlStr) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];

        gDelegate = [[STAppDelegate alloc] init];
        gDelegate.initialURL = [NSString stringWithUTF8String:urlStr];
        [NSApp setDelegate:gDelegate];
        gNativeLoopRunning = YES;
        [NSApp run];
        gNativeLoopRunning = NO;
    }
}

static int isNativeMacOSLoopRunning(void) {
    return gNativeLoopRunning ? 1 : 0;
}

static char *pickNativeMacOSBookPath(int chooseDir) {
    if (!gNativeLoopRunning) {
        return NULL;
    }
    __block char *result = NULL;
    void (^presentBlock)(void) = ^{
        @autoreleasepool {
            NSOpenPanel *panel = [NSOpenPanel openPanel];
            [panel setAllowsMultipleSelection:NO];
            if (chooseDir) {
                [panel setCanChooseDirectories:YES];
                [panel setCanChooseFiles:NO];
                [panel setCanCreateDirectories:YES];
                [panel setTitle:@"Wybierz folder na dysku"];
                [panel setPrompt:@"Wybierz folder"];
            } else {
                [panel setCanChooseDirectories:YES];
                [panel setCanChooseFiles:YES];
                [panel setTitle:@"Wybierz książkę (.stbook / .zip lub folder)"];
                [panel setPrompt:@"Dodaj książkę"];
            }
            NSModalResponse resp = [panel runModal];
            if (resp == NSModalResponseOK && [panel URL] != nil) {
                NSString *path = [[panel URL] path];
                if (path != nil && [path length] > 0) {
                    result = strdup([path UTF8String]);
                }
            }
        }
    };
    if ([NSThread isMainThread]) {
        presentBlock();
    } else {
        dispatch_sync(dispatch_get_main_queue(), presentBlock);
    }
    return result;
}

static char *pickNativeMacOSMediaPaths(const char *kindCStr) {
    if (!gNativeLoopRunning) {
        return NULL;
    }
    __block char *result = NULL;
    NSString *kind = kindCStr != NULL ? [NSString stringWithUTF8String:kindCStr] : @"";
    void (^presentBlock)(void) = ^{
        @autoreleasepool {
            NSOpenPanel *panel = [NSOpenPanel openPanel];
            [panel setAllowsMultipleSelection:YES];
            [panel setCanChooseDirectories:NO];
            [panel setCanChooseFiles:YES];
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
            if ([kind isEqualToString:@"image"] || [kind isEqualToString:@"img"]) {
                [panel setTitle:@"Wybierz obraz z dysku (.png, .jpg, .webp)"];
                [panel setPrompt:@"Dodaj obraz"];
                [panel setAllowedFileTypes:@[@"png", @"jpg", @"jpeg", @"webp", @"PNG", @"JPG", @"JPEG", @"WEBP"]];
            } else if ([kind isEqualToString:@"audio"] || [kind isEqualToString:@"voice"]) {
                [panel setTitle:@"Wybierz nagranie audio z dysku (.ogg, .flac, .mp3)"];
                [panel setPrompt:@"Dodaj audio"];
                [panel setAllowedFileTypes:@[@"ogg", @"flac", @"mp3", @"OGG", @"FLAC", @"MP3"]];
            } else {
                [panel setTitle:@"Wybierz pliki multimedialne z dysku (obrazy lub audio)"];
                [panel setPrompt:@"Dodaj media"];
                [panel setAllowedFileTypes:@[@"png", @"jpg", @"jpeg", @"webp", @"ogg", @"flac", @"mp3", @"PNG", @"JPG", @"JPEG", @"WEBP", @"OGG", @"FLAC", @"MP3"]];
            }
#pragma clang diagnostic pop
            NSModalResponse resp = [panel runModal];
            if (resp == NSModalResponseOK && [[panel URLs] count] > 0) {
                NSMutableArray<NSString *> *paths = [NSMutableArray array];
                for (NSURL *u in [panel URLs]) {
                    NSString *p = [u path];
                    if (p != nil && [p length] > 0) {
                        [paths addObject:p];
                    }
                }
                if ([paths count] > 0) {
                    NSString *joined = [paths componentsJoinedByString:@"\n"];
                    result = strdup([joined UTF8String]);
                }
            }
        }
    };
    if ([NSThread isMainThread]) {
        presentBlock();
    } else {
        dispatch_sync(dispatch_get_main_queue(), presentBlock);
    }
    return result;
}

static void toggleNativeMacOSFullscreen(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gDelegate != nil && gDelegate.window != nil) {
            [gDelegate.window toggleFullScreen:nil];
        }
    });
}

static void stopNativeMacOSWebView(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp terminate:nil];
    });
}
*/
import "C"
import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"unsafe"
)

func init() {
	// Cocoa NSApplication must run on the main OS thread.
	runtime.LockOSThread()
}

func RunNativeOSWindow(url string, onClosed func()) bool {
	if os.Getenv("ST_NO_WINDOW") == "1" {
		return false
	}
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))

	C.runNativeMacOSWebView(cURL)
	if onClosed != nil {
		onClosed()
	}
	return true
}

func PickNativeBookPath(chooseDir bool) string {
	if os.Getenv("ST_NO_WINDOW") == "1" || strings.HasSuffix(os.Args[0], ".test") {
		return ""
	}
	if C.isNativeMacOSLoopRunning() != 0 {
		dirFlag := C.int(0)
		if chooseDir {
			dirFlag = C.int(1)
		}
		if cPath := C.pickNativeMacOSBookPath(dirFlag); cPath != nil {
			defer C.free(unsafe.Pointer(cPath))
			return strings.TrimSpace(C.GoString(cPath))
		}
		return ""
	}
	if osascript, err := exec.LookPath("osascript"); err == nil && osascript != "" {
		script := `POSIX path of (choose file with prompt "Wybierz książkę (.stbook / .zip):")`
		if chooseDir {
			script = `POSIX path of (choose folder with prompt "Wybierz folder książki z dysku:")`
		}
		if out, err := exec.Command(osascript, "-e", script).Output(); err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

func PickNativeMediaPaths(kind string) []string {
	if os.Getenv("ST_NO_WINDOW") == "1" || strings.HasSuffix(os.Args[0], ".test") {
		return nil
	}
	normKind := strings.ToLower(strings.TrimSpace(kind))
	if C.isNativeMacOSLoopRunning() != 0 {
		cKind := C.CString(normKind)
		defer C.free(unsafe.Pointer(cKind))
		if cPaths := C.pickNativeMacOSMediaPaths(cKind); cPaths != nil {
			defer C.free(unsafe.Pointer(cPaths))
			raw := strings.TrimSpace(C.GoString(cPaths))
			if raw != "" {
				var out []string
				for _, p := range strings.Split(raw, "\n") {
					if trimmed := strings.TrimSpace(p); trimmed != "" {
						out = append(out, trimmed)
					}
				}
				return out
			}
		}
		return nil
	}
	if osascript, err := exec.LookPath("osascript"); err == nil && osascript != "" {
		script := `POSIX path of (choose file with prompt "Wybierz plik multimedialny z dysku:")`
		switch normKind {
		case "image", "img":
			script = `POSIX path of (choose file of type {"png", "jpg", "jpeg", "webp"} with prompt "Wybierz obraz z dysku (.png, .jpg, .webp):")`
		case "audio", "voice":
			script = `POSIX path of (choose file of type {"ogg", "flac", "mp3"} with prompt "Wybierz nagranie audio z dysku (.ogg, .flac, .mp3):")`
		}
		if out, err := exec.Command(osascript, "-e", script).Output(); err == nil {
			if trimmed := strings.TrimSpace(string(out)); trimmed != "" {
				return []string{trimmed}
			}
		}
	}
	return nil
}

func ToggleNativeWindowFullscreen() {
	C.toggleNativeMacOSFullscreen()
}

func StartNativeWindowDrag() {}

func TerminateNativeWindow() {
	C.stopNativeMacOSWebView()
}
