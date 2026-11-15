//go:build darwin && cgo

package osbridge

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AudioToolbox -framework CoreAudio -framework AVFoundation -framework Foundation

#import <Foundation/Foundation.h>
#import <AVFoundation/AVFoundation.h>
#import <AudioToolbox/AudioToolbox.h>
#include <pthread.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>

#define ST_VOICE_SAMPLE_RATE 24000
#define ST_VOICE_NUM_BUFFERS 3
#define ST_VOICE_BUF_SAMPLES 1200

static pthread_mutex_t gVoiceCaptureMu = PTHREAD_MUTEX_INITIALIZER;
static AudioQueueRef gVoiceQueue = NULL;
static AudioQueueBufferRef gVoiceBuffers[ST_VOICE_NUM_BUFFERS];
static int16_t *gVoicePCMBuf = NULL;
static size_t gVoicePCMLen = 0;
static size_t gVoicePCMCap = 0;
static float gVoiceLastPeak = 0.0f;
static int gVoiceIsCapturing = 0;

static void stAudioInputCallback(
    void *inUserData,
    AudioQueueRef inAQ,
    AudioQueueBufferRef inBuffer,
    const AudioTimeStamp *inStartTime,
    UInt32 inNumberPacketDescriptions,
    const AudioStreamPacketDescription *inPacketDescs
) {
    size_t numSamples = (inBuffer != NULL) ? (inBuffer->mAudioDataByteSize / sizeof(int16_t)) : 0;
    const int16_t *samples = (inBuffer != NULL && inBuffer->mAudioData != NULL) ? (const int16_t *)inBuffer->mAudioData : NULL;
    float scaled = 0.0f;
    if (numSamples > 0 && samples != NULL) {
        float maxAbs = 0.0f;
        for (size_t i = 0; i < numSamples; i++) {
            float a = fabsf((float)samples[i] / 32768.0f);
            if (a > maxAbs) {
                maxAbs = a;
            }
        }
        scaled = maxAbs * 1.8f;
        if (scaled > 1.0f) scaled = 1.0f;
    }

    pthread_mutex_lock(&gVoiceCaptureMu);
    if (!gVoiceIsCapturing || gVoiceQueue != inAQ) {
        pthread_mutex_unlock(&gVoiceCaptureMu);
        return;
    }
    if (numSamples > 0 && samples != NULL) {
        gVoiceLastPeak = scaled;

        // Cap single recording at 30 minutes (24000 * 1800 samples)
        size_t maxSamples = ST_VOICE_SAMPLE_RATE * 1800;
        if (gVoicePCMLen + numSamples <= maxSamples) {
            if (gVoicePCMLen + numSamples > gVoicePCMCap) {
                size_t newCap = gVoicePCMCap == 0 ? (ST_VOICE_SAMPLE_RATE * 60) : (gVoicePCMCap * 2);
                while (newCap < gVoicePCMLen + numSamples) {
                    newCap *= 2;
                }
                int16_t *nextBuf = (int16_t *)realloc(gVoicePCMBuf, newCap * sizeof(int16_t));
                if (nextBuf != NULL) {
                    gVoicePCMBuf = nextBuf;
                    gVoicePCMCap = newCap;
                }
            }
            if (gVoicePCMBuf != NULL && gVoicePCMLen + numSamples <= gVoicePCMCap) {
                memcpy(gVoicePCMBuf + gVoicePCMLen, samples, numSamples * sizeof(int16_t));
                gVoicePCMLen += numSamples;
            }
        }
    }
    if (inAQ != NULL && inBuffer != NULL) {
        AudioQueueEnqueueBuffer(inAQ, inBuffer, 0, NULL);
    }
    pthread_mutex_unlock(&gVoiceCaptureMu);
}

static int stRequestMicPermissionIfNeeded(void) {
    if (@available(macOS 10.14, *)) {
        AVAuthorizationStatus status = [AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
        if (status == AVAuthorizationStatusAuthorized) {
            return 1;
        }
        if (status == AVAuthorizationStatusDenied || status == AVAuthorizationStatusRestricted) {
            return 0;
        }
        dispatch_semaphore_t sem = dispatch_semaphore_create(0);
        __block BOOL granted = NO;
        [AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio completionHandler:^(BOOL ok) {
            granted = ok;
            dispatch_semaphore_signal(sem);
        }];
        dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, (int64_t)(15 * NSEC_PER_SEC)));
        return granted ? 1 : 0;
    }
    return 1;
}

static int stStartNativeAudioCapture(void) {
    if (!stRequestMicPermissionIfNeeded()) {
        return -2;
    }
    pthread_mutex_lock(&gVoiceCaptureMu);
    AudioQueueRef staleQueue = gVoiceQueue;
    gVoiceQueue = NULL;
    gVoiceIsCapturing = 0;
    size_t minInitialCap = ST_VOICE_SAMPLE_RATE * 60;
    if (gVoicePCMCap < minInitialCap) {
        int16_t *nextBuf = (int16_t *)realloc(gVoicePCMBuf, minInitialCap * sizeof(int16_t));
        if (nextBuf != NULL) {
            gVoicePCMBuf = nextBuf;
            gVoicePCMCap = minInitialCap;
        }
    }
    gVoicePCMLen = 0;
    gVoiceLastPeak = 0.0f;
    pthread_mutex_unlock(&gVoiceCaptureMu);

    if (staleQueue != NULL) {
        AudioQueueStop(staleQueue, true);
        AudioQueueDispose(staleQueue, true);
    }

    AudioStreamBasicDescription fmt;
    memset(&fmt, 0, sizeof(fmt));
    fmt.mSampleRate = ST_VOICE_SAMPLE_RATE;
    fmt.mFormatID = kAudioFormatLinearPCM;
    fmt.mFormatFlags = kLinearPCMFormatFlagIsSignedInteger | kLinearPCMFormatFlagIsPacked;
    fmt.mFramesPerPacket = 1;
    fmt.mChannelsPerFrame = 1;
    fmt.mBitsPerChannel = 16;
    fmt.mBytesPerFrame = 2;
    fmt.mBytesPerPacket = 2;

    AudioQueueRef queue = NULL;
    OSStatus err = AudioQueueNewInput(&fmt, stAudioInputCallback, NULL, NULL, NULL, 0, &queue);
    if (err != noErr || queue == NULL) {
        return -1;
    }
    UInt32 bufByteSize = ST_VOICE_BUF_SAMPLES * sizeof(int16_t);
    for (int i = 0; i < ST_VOICE_NUM_BUFFERS; i++) {
        gVoiceBuffers[i] = NULL;
        err = AudioQueueAllocateBuffer(queue, bufByteSize, &gVoiceBuffers[i]);
        if (err == noErr && gVoiceBuffers[i] != NULL) {
            AudioQueueEnqueueBuffer(queue, gVoiceBuffers[i], 0, NULL);
        }
    }

    pthread_mutex_lock(&gVoiceCaptureMu);
    gVoiceQueue = queue;
    gVoiceIsCapturing = 1;
    pthread_mutex_unlock(&gVoiceCaptureMu);

    err = AudioQueueStart(queue, NULL);
    if (err != noErr) {
        pthread_mutex_lock(&gVoiceCaptureMu);
        if (gVoiceQueue == queue) {
            gVoiceIsCapturing = 0;
            gVoiceQueue = NULL;
        }
        pthread_mutex_unlock(&gVoiceCaptureMu);
        AudioQueueDispose(queue, true);
        return -1;
    }
    return 0;
}

static float stPollNativeAudioCapturePeak(int *outActive) {
    pthread_mutex_lock(&gVoiceCaptureMu);
    if (outActive != NULL) {
        *outActive = gVoiceIsCapturing;
    }
    float p = gVoiceLastPeak;
    pthread_mutex_unlock(&gVoiceCaptureMu);
    return p;
}

static uint8_t *stStopNativeAudioCapture(size_t *outByteLen, int *outSampleRate) {
    pthread_mutex_lock(&gVoiceCaptureMu);
    gVoiceIsCapturing = 0;
    AudioQueueRef queue = gVoiceQueue;
    gVoiceQueue = NULL;
    pthread_mutex_unlock(&gVoiceCaptureMu);

    if (queue != NULL) {
        AudioQueueStop(queue, true);
        AudioQueueDispose(queue, true);
    }

    pthread_mutex_lock(&gVoiceCaptureMu);
    if (outSampleRate != NULL) {
        *outSampleRate = ST_VOICE_SAMPLE_RATE;
    }
    size_t byteLen = gVoicePCMLen * sizeof(int16_t);
    if (byteLen == 0 || gVoicePCMBuf == NULL) {
        if (outByteLen != NULL) *outByteLen = 0;
        pthread_mutex_unlock(&gVoiceCaptureMu);
        return NULL;
    }
    uint8_t *copyBuf = (uint8_t *)malloc(byteLen);
    if (copyBuf != NULL) {
        memcpy(copyBuf, gVoicePCMBuf, byteLen);
        if (outByteLen != NULL) *outByteLen = byteLen;
    } else {
        if (outByteLen != NULL) *outByteLen = 0;
    }
    gVoicePCMLen = 0;
    gVoiceLastPeak = 0.0f;
    pthread_mutex_unlock(&gVoiceCaptureMu);
    return copyBuf;
}
*/
import "C"
import (
	"fmt"
	"os"
	"strings"
	"unsafe"
)

func StartNativeAudioCapture() error {
	if os.Getenv("ST_NO_WINDOW") == "1" || strings.HasSuffix(os.Args[0], ".test") {
		return nil
	}
	rc := int(C.stStartNativeAudioCapture())
	if rc == -2 {
		return fmt.Errorf("brak uprawnień do mikrofonu w ustawieniach macOS")
	}
	if rc != 0 {
		return fmt.Errorf("nie udało się uruchomić nagrywania z mikrofonu")
	}
	return nil
}

func PollNativeAudioCapturePeak() (float64, bool) {
	if os.Getenv("ST_NO_WINDOW") == "1" || strings.HasSuffix(os.Args[0], ".test") {
		return 0.35, true
	}
	var active C.int
	peak := float64(C.stPollNativeAudioCapturePeak(&active))
	return peak, active != 0
}

func StopNativeAudioCapture() ([]byte, int, error) {
	if os.Getenv("ST_NO_WINDOW") == "1" || strings.HasSuffix(os.Args[0], ".test") {
		sr := 24000
		fake := make([]byte, sr*2)
		return fake, sr, nil
	}
	var byteLen C.size_t
	var sr C.int
	ptr := C.stStopNativeAudioCapture(&byteLen, &sr)
	if ptr == nil || byteLen == 0 {
		return nil, int(sr), fmt.Errorf("brak zarejestrowanych próbek audio z mikrofonu")
	}
	defer C.free(unsafe.Pointer(ptr))
	pcm := C.GoBytes(unsafe.Pointer(ptr), C.int(byteLen))
	return pcm, int(sr), nil
}
