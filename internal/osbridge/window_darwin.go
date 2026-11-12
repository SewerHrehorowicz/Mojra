//go:build darwin && cgo

package osbridge

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

extern void goLogCocoaStartupStage(int stageCode, char *detail);

#ifndef GO_CGO_EXPORT_PROLOGUE_H

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

@interface STAppDelegate : NSObject <NSApplicationDelegate, NSWindowDelegate, WKScriptMessageHandler, WKUIDelegate, WKNavigationDelegate>
@property (nonatomic, strong) STWindow *window;
@property (nonatomic, strong) WKWebView *webView;
@property (nonatomic, copy) NSString *initialURL;
@property (assign) const void *iconBytes;
@property (assign) int iconLen;
@end

@implementation STAppDelegate

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
            if (![btn isHidden]) {
                [btn setHidden:YES];
            }
            if ([btn isEnabled]) {
                [btn setEnabled:NO];
            }
            NSView *titlebarView = [btn superview];
            if (titlebarView != nil) {
                if (![titlebarView isHidden]) {
                    [titlebarView setHidden:YES];
                }
                NSView *titlebarContainer = [titlebarView superview];
                if (titlebarContainer != nil && ![titlebarContainer isHidden]) {
                    [titlebarContainer setHidden:YES];
                }
            }
        }
    }
    NSView *themeFrame = [[self.window contentView] superview];
    if (themeFrame != nil) {
        for (NSView *sub in [themeFrame subviews]) {
            if (sub != [self.window contentView] && [NSStringFromClass([sub class]) containsString:@"Titlebar"]) {
                if (![sub isHidden]) {
                    [sub setHidden:YES];
                }
            }
        }
    }
    for (NSWindow *win in [NSApp windows]) {
        if (win != self.window && [NSStringFromClass([win class]) containsString:@"ToolbarFullScreen"]) {
            if (![win ignoresMouseEvents]) {
                [win setIgnoresMouseEvents:YES];
            }
            if ([win isOpaque]) {
                [win setOpaque:NO];
            }
            if ([win hasShadow]) {
                [win setHasShadow:NO];
            }
            if ([win alphaValue] != 0.0) {
                [win setBackgroundColor:[NSColor clearColor]];
                [win setAlphaValue:0.0];
            }
            if ([win contentView] != nil && ![[win contentView] isHidden]) {
                [[win contentView] setHidden:YES];
            }
        }
    }
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
    goLogCocoaStartupStage(1, (char *)[self.initialURL UTF8String]);
    [[NSProcessInfo processInfo] setProcessName:@"Mojra"];

    NSUserDefaults *defaults = [NSUserDefaults standardUserDefaults];
    NSArray<NSString *> *offKeys = @[
        @"WebAutomaticSpellingCorrectionEnabled",
        @"WebContinuousSpellCheckingEnabled",
        @"WebAutomaticQuoteSubstitutionEnabled",
        @"WebAutomaticDashSubstitutionEnabled",
        @"WebAutomaticTextReplacementEnabled",
        @"NSAutomaticSpellingCorrectionEnabled",
        @"NSAutomaticQuoteSubstitutionEnabled",
        @"NSAutomaticDashSubstitutionEnabled",
        @"NSAutomaticTextReplacementEnabled",
        @"NSAutomaticCapitalizationEnabled",
        @"NSAutomaticPeriodSubstitutionEnabled"
    ];
    for (NSString *key in offKeys) {
        [defaults setBool:NO forKey:key];
    }

    NSMenu *mainMenu = [[NSMenu alloc] init];
    NSMenuItem *appMenuItem = [[NSMenuItem alloc] init];
    [mainMenu addItem:appMenuItem];
    NSMenu *appMenu = [[NSMenu alloc] initWithTitle:@"Mojra"];
    [appMenu addItemWithTitle:@"Zakończ Mojra" action:@selector(terminate:) keyEquivalent:@"q"];
    [appMenuItem setSubmenu:appMenu];

    NSMenuItem *editMenuItem = [[NSMenuItem alloc] init];
    [mainMenu addItem:editMenuItem];
    NSMenu *editMenu = [[NSMenu alloc] initWithTitle:@"Edycja"];
    [editMenu addItemWithTitle:@"Cofnij" action:@selector(undo:) keyEquivalent:@"z"];
    NSMenuItem *redoItem = [editMenu addItemWithTitle:@"Ponów" action:@selector(redo:) keyEquivalent:@"Z"];
    [redoItem setKeyEquivalentModifierMask:(NSEventModifierFlagCommand | NSEventModifierFlagShift)];
    [editMenu addItem:[NSMenuItem separatorItem]];
    [editMenu addItemWithTitle:@"Wytnij" action:@selector(cut:) keyEquivalent:@"x"];
    [editMenu addItemWithTitle:@"Kopiuj" action:@selector(copy:) keyEquivalent:@"c"];
    [editMenu addItemWithTitle:@"Wklej" action:@selector(paste:) keyEquivalent:@"v"];
    [editMenu addItemWithTitle:@"Zaznacz wszystko" action:@selector(selectAll:) keyEquivalent:@"a"];
    [editMenuItem setSubmenu:editMenu];
    [NSApp setMainMenu:mainMenu];

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
    [self.window setTitle:@"Mojra"];
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
    [[config userContentController] addScriptMessageHandler:self name:@"nativeHost"];

    self.webView = [[WKWebView alloc] initWithFrame:[[self.window contentView] bounds]
                                      configuration:config];
    [self.webView setUIDelegate:self];
    [self.webView setNavigationDelegate:self];
    [self.webView setAutoresizingMask:(NSViewWidthSizable | NSViewHeightSizable)];
    [self.webView setValue:@NO forKey:@"drawsBackground"];
    goLogCocoaStartupStage(3, (char *)[self.initialURL UTF8String]);

    [[self.window contentView] addSubview:self.webView];

    NSURL *url = [NSURL URLWithString:self.initialURL];
    NSURLRequest *req = [NSURLRequest requestWithURL:url];
    [self.webView loadRequest:req];
    goLogCocoaStartupStage(4, (char *)[self.initialURL UTF8String]);

    [self.window makeKeyAndOrderFront:nil];
    [self hideSystemWindowButtons];
    [NSApp activateIgnoringOtherApps:YES];
    if ([[NSBundle mainBundle] bundleIdentifier] == nil && self.iconBytes != NULL && self.iconLen > 0) {
        NSData *data = [NSData dataWithBytes:self.iconBytes length:(NSUInteger)self.iconLen];
        NSImage *icon = [[NSImage alloc] initWithData:data];
        if (icon != nil) {
            [NSApp setApplicationIconImage:icon];
        }
        goLogCocoaStartupStage(2, (char *)"icon.png");
    }
}

- (void)webView:(WKWebView *)webView didCommitNavigation:(WKNavigation *)navigation {
    goLogCocoaStartupStage(5, (char *)[[[webView URL] absoluteString] UTF8String]);
}

- (void)webView:(WKWebView *)webView didFinishNavigation:(WKNavigation *)navigation {
    goLogCocoaStartupStage(6, (char *)[[[webView URL] absoluteString] UTF8String]);
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

- (void)webView:(WKWebView *)webView
    requestMediaCapturePermissionForOrigin:(WKSecurityOrigin *)origin
                          initiatedByFrame:(WKFrameInfo *)frame
                                      type:(WKMediaCaptureType)type
                           decisionHandler:(void (^)(WKPermissionDecision decision))decisionHandler API_AVAILABLE(macos(12.0)) {
    NSString *host = [[origin host] lowercaseString];
    if ([host isEqualToString:@"127.0.0.1"] || [host isEqualToString:@"localhost"]) {
        decisionHandler(WKPermissionDecisionGrant);
        return;
    }
    decisionHandler(WKPermissionDecisionPrompt);
}

@end

static STAppDelegate *gDelegate = nil;
static BOOL gNativeLoopRunning = NO;

static void runNativeMacOSWebView(const char *urlStr, const void *iconBytes, int iconLen) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];

        gDelegate = [[STAppDelegate alloc] init];
        gDelegate.initialURL = [NSString stringWithUTF8String:urlStr];
        gDelegate.iconBytes = iconBytes;
        gDelegate.iconLen = iconLen;
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
                [panel setTitle:@"Wybierz książkę (.playbook / .zip lub folder)"];
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

#endif
*/
import "C"
import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"mojra"
	"mojra/internal/logging"
)

const embeddedAppIconPath = "app/icons/icon.png"

var _ = goLogCocoaStartupStage

//export goLogCocoaStartupStage
func goLogCocoaStartupStage(stageCode C.int, detail *C.char) {
	if !logging.Active.Enabled {
		return
	}
	var d string
	if detail != nil {
		d = C.GoString(detail)
	}
	switch stageCode {
	case 1:
		logging.Active.MarkStep(logging.StageCocoaAppLaunch, time.Time{}, d)
	case 2:
		logging.Active.MarkStep(logging.StageCocoaIconDraw, time.Time{}, d)
	case 3:
		logging.Active.MarkStep(logging.StageCocoaWebViewInit, time.Time{}, d)
	case 4:
		logging.Active.MarkStep(logging.StageCocoaLoadRequest, time.Time{}, d)
	case 5:
		logging.Active.MarkStep(logging.StageCocoaNavCommit, time.Time{}, d)
	case 6:
		logging.Active.MarkStep(logging.StageCocoaNavFinish, time.Time{}, d)
	}
}

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

	var iconPtr unsafe.Pointer
	var iconLen C.int
	if iconData, err := mojra.AppFS.ReadFile(embeddedAppIconPath); err == nil && len(iconData) > 0 {
		iconPtr = unsafe.Pointer(&iconData[0])
		iconLen = C.int(len(iconData))
	}

	C.runNativeMacOSWebView(cURL, iconPtr, iconLen)
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
		script := `POSIX path of (choose file with prompt "Wybierz książkę (.playbook / .zip):")`
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
