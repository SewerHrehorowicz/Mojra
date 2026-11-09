//go:build android && cgo

package gui

/*
#cgo LDFLAGS: -lOpenSLES

#include <SLES/OpenSLES.h>
#include <SLES/OpenSLES_Android.h>
#include <pthread.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>
#include <stdint.h>

#define ST_ANDROID_BUF_SAMPLES 1200

static pthread_mutex_t gAndroidMicMu = PTHREAD_MUTEX_INITIALIZER;
static SLObjectItf gEngineObj = NULL;
static SLEngineItf gEngineItf = NULL;
static SLObjectItf gRecorderObj = NULL;
static SLRecordItf gRecorderItf = NULL;
static SLAndroidSimpleBufferQueueItf gRecorderBufQ = NULL;
static int16_t gAndroidQueueBufs[2][ST_ANDROID_BUF_SAMPLES];
static int gAndroidNextBufIdx = 0;
static int16_t *gAndroidPCMData = NULL;
static size_t gAndroidPCMCount = 0;
static size_t gAndroidPCMCap = 0;
static int gAndroidSampleRate = 24000;
static float gAndroidPeak = 0.0f;
static int gAndroidActive = 0;

static void stAndroidReleaseObjects(void) {
    if (gRecorderObj != NULL) {
        (*gRecorderObj)->Destroy(gRecorderObj);
        gRecorderObj = NULL;
        gRecorderItf = NULL;
        gRecorderBufQ = NULL;
    }
    if (gEngineObj != NULL) {
        (*gEngineObj)->Destroy(gEngineObj);
        gEngineObj = NULL;
        gEngineItf = NULL;
    }
}

static void SLAPIENTRY stAndroidMicCallback(SLAndroidSimpleBufferQueueItf bq, void *ctx) {
    (void)ctx;
    int idx = gAndroidNextBufIdx;
    gAndroidNextBufIdx = (idx + 1) & 1;
    const int16_t *samples = gAndroidQueueBufs[idx];

    float maxAbs = 0.0f;
    for (size_t i = 0; i < ST_ANDROID_BUF_SAMPLES; i++) {
        float a = fabsf((float)samples[i] / 32768.0f);
        if (a > maxAbs) {
            maxAbs = a;
        }
    }
    float scaled = maxAbs * 1.8f;
    if (scaled > 1.0f) {
        scaled = 1.0f;
    }

    pthread_mutex_lock(&gAndroidMicMu);
    if (!gAndroidActive || bq != gRecorderBufQ) {
        pthread_mutex_unlock(&gAndroidMicMu);
        return;
    }
    gAndroidPeak = scaled;
    size_t limitSamples = (size_t)gAndroidSampleRate * 1800;
    if (gAndroidPCMCount + ST_ANDROID_BUF_SAMPLES <= limitSamples) {
        if (gAndroidPCMCount + ST_ANDROID_BUF_SAMPLES > gAndroidPCMCap) {
            size_t nextCap = gAndroidPCMCap == 0 ? ((size_t)gAndroidSampleRate * 60) : (gAndroidPCMCap * 2);
            while (nextCap < gAndroidPCMCount + ST_ANDROID_BUF_SAMPLES) {
                nextCap *= 2;
            }
            int16_t *grown = (int16_t *)realloc(gAndroidPCMData, nextCap * sizeof(int16_t));
            if (grown != NULL) {
                gAndroidPCMData = grown;
                gAndroidPCMCap = nextCap;
            }
        }
        if (gAndroidPCMData != NULL && gAndroidPCMCount + ST_ANDROID_BUF_SAMPLES <= gAndroidPCMCap) {
            memcpy(gAndroidPCMData + gAndroidPCMCount, samples, ST_ANDROID_BUF_SAMPLES * sizeof(int16_t));
            gAndroidPCMCount += ST_ANDROID_BUF_SAMPLES;
        }
    }
    pthread_mutex_unlock(&gAndroidMicMu);

    if (bq != NULL) {
        (*bq)->Enqueue(bq, gAndroidQueueBufs[idx], sizeof(gAndroidQueueBufs[idx]));
    }
}

static int stAndroidStartMic(void) {
    pthread_mutex_lock(&gAndroidMicMu);
    if (gAndroidActive && gRecorderObj != NULL) {
        gAndroidPCMCount = 0;
        gAndroidPeak = 0.0f;
        pthread_mutex_unlock(&gAndroidMicMu);
        return 0;
    }
    gAndroidActive = 0;
    gAndroidPCMCount = 0;
    gAndroidPeak = 0.0f;
    pthread_mutex_unlock(&gAndroidMicMu);

    stAndroidReleaseObjects();

    SLresult res = slCreateEngine(&gEngineObj, 0, NULL, 0, NULL, NULL);
    if (res != SL_RESULT_SUCCESS || gEngineObj == NULL) {
        return -1;
    }
    res = (*gEngineObj)->Realize(gEngineObj, SL_BOOLEAN_FALSE);
    if (res != SL_RESULT_SUCCESS) {
        stAndroidReleaseObjects();
        return -1;
    }
    res = (*gEngineObj)->GetInterface(gEngineObj, SL_IID_ENGINE, &gEngineItf);
    if (res != SL_RESULT_SUCCESS || gEngineItf == NULL) {
        stAndroidReleaseObjects();
        return -1;
    }

    SLDataLocator_IODevice locDev = {
        SL_DATALOCATOR_IODEVICE,
        SL_IODEVICE_AUDIOINPUT,
        SL_DEFAULTDEVICEID_AUDIOINPUT,
        NULL
    };
    SLDataSource audioSrc = { &locDev, NULL };
    SLDataLocator_AndroidSimpleBufferQueue locBQ = {
        SL_DATALOCATOR_ANDROIDSIMPLEBUFFERQUEUE,
        2
    };

    const SLuint32 candidateSLRates[4] = {
        SL_SAMPLINGRATE_24,
        SL_SAMPLINGRATE_16,
        SL_SAMPLINGRATE_44_1,
        SL_SAMPLINGRATE_48
    };
    const int candidateHz[4] = { 24000, 16000, 44100, 48000 };
    int chosenHz = 24000;
    int created = 0;

    for (int i = 0; i < 4; i++) {
        SLDataFormat_PCM fmtPCM = {
            SL_DATAFORMAT_PCM,
            1,
            candidateSLRates[i],
            SL_PCMSAMPLEFORMAT_FIXED_16,
            SL_PCMSAMPLEFORMAT_FIXED_16,
            SL_SPEAKER_FRONT_CENTER,
            SL_BYTEORDER_LITTLEENDIAN
        };
        SLDataSink audioSnk = { &locBQ, &fmtPCM };
        const SLInterfaceID iids[1] = { SL_IID_ANDROIDSIMPLEBUFFERQUEUE };
        const SLboolean reqs[1] = { SL_BOOLEAN_TRUE };

        res = (*gEngineItf)->CreateAudioRecorder(gEngineItf, &gRecorderObj, &audioSrc, &audioSnk, 1, iids, reqs);
        if (res == SL_RESULT_SUCCESS && gRecorderObj != NULL) {
            res = (*gRecorderObj)->Realize(gRecorderObj, SL_BOOLEAN_FALSE);
            if (res == SL_RESULT_SUCCESS) {
                chosenHz = candidateHz[i];
                created = 1;
                break;
            }
            (*gRecorderObj)->Destroy(gRecorderObj);
            gRecorderObj = NULL;
        }
    }

    if (!created || gRecorderObj == NULL) {
        stAndroidReleaseObjects();
        return -2;
    }

    res = (*gRecorderObj)->GetInterface(gRecorderObj, SL_IID_RECORD, &gRecorderItf);
    if (res != SL_RESULT_SUCCESS || gRecorderItf == NULL) {
        stAndroidReleaseObjects();
        return -1;
    }
    res = (*gRecorderObj)->GetInterface(gRecorderObj, SL_IID_ANDROIDSIMPLEBUFFERQUEUE, &gRecorderBufQ);
    if (res != SL_RESULT_SUCCESS || gRecorderBufQ == NULL) {
        stAndroidReleaseObjects();
        return -1;
    }
    res = (*gRecorderBufQ)->RegisterCallback(gRecorderBufQ, stAndroidMicCallback, NULL);
    if (res != SL_RESULT_SUCCESS) {
        stAndroidReleaseObjects();
        return -1;
    }

    memset(gAndroidQueueBufs, 0, sizeof(gAndroidQueueBufs));
    gAndroidNextBufIdx = 0;
    (*gRecorderBufQ)->Enqueue(gRecorderBufQ, gAndroidQueueBufs[0], sizeof(gAndroidQueueBufs[0]));
    (*gRecorderBufQ)->Enqueue(gRecorderBufQ, gAndroidQueueBufs[1], sizeof(gAndroidQueueBufs[1]));

    pthread_mutex_lock(&gAndroidMicMu);
    gAndroidSampleRate = chosenHz;
    size_t minCap = (size_t)chosenHz * 60;
    if (gAndroidPCMCap < minCap) {
        int16_t *grown = (int16_t *)realloc(gAndroidPCMData, minCap * sizeof(int16_t));
        if (grown != NULL) {
            gAndroidPCMData = grown;
            gAndroidPCMCap = minCap;
        }
    }
    gAndroidPCMCount = 0;
    gAndroidPeak = 0.0f;
    gAndroidActive = 1;
    pthread_mutex_unlock(&gAndroidMicMu);

    res = (*gRecorderItf)->SetRecordState(gRecorderItf, SL_RECORDSTATE_RECORDING);
    if (res != SL_RESULT_SUCCESS) {
        pthread_mutex_lock(&gAndroidMicMu);
        gAndroidActive = 0;
        pthread_mutex_unlock(&gAndroidMicMu);
        stAndroidReleaseObjects();
        return -2;
    }
    return 0;
}

static float stAndroidPollPeak(int *outActive) {
    pthread_mutex_lock(&gAndroidMicMu);
    if (outActive != NULL) {
        *outActive = gAndroidActive;
    }
    float p = gAndroidPeak;
    pthread_mutex_unlock(&gAndroidMicMu);
    return p;
}

static uint8_t *stAndroidStopMic(size_t *outByteLen, int *outSampleRate) {
    pthread_mutex_lock(&gAndroidMicMu);
    gAndroidActive = 0;
    pthread_mutex_unlock(&gAndroidMicMu);

    if (gRecorderItf != NULL) {
        (*gRecorderItf)->SetRecordState(gRecorderItf, SL_RECORDSTATE_STOPPED);
    }
    if (gRecorderBufQ != NULL) {
        (*gRecorderBufQ)->Clear(gRecorderBufQ);
    }
    stAndroidReleaseObjects();

    pthread_mutex_lock(&gAndroidMicMu);
    if (outSampleRate != NULL) {
        *outSampleRate = gAndroidSampleRate;
    }
    size_t byteLen = gAndroidPCMCount * sizeof(int16_t);
    uint8_t *outBuf = NULL;
    if (byteLen > 0 && gAndroidPCMData != NULL) {
        outBuf = (uint8_t *)malloc(byteLen);
        if (outBuf != NULL) {
            memcpy(outBuf, gAndroidPCMData, byteLen);
            if (outByteLen != NULL) {
                *outByteLen = byteLen;
            }
        }
    }
    if (outBuf == NULL && outByteLen != NULL) {
        *outByteLen = 0;
    }
    gAndroidPCMCount = 0;
    gAndroidPeak = 0.0f;
    pthread_mutex_unlock(&gAndroidMicMu);
    return outBuf;
}
*/
import "C"
import (
	"fmt"
	"os"
	"strings"
	"unsafe"
)

func isAndroidTestAudioMock() bool {
	return strings.HasSuffix(os.Args[0], ".test") ||
		(os.Getenv("ST_NO_WINDOW") == "1" && os.Getenv("MOJRA_ANDROID_APK") != "1")
}

func StartNativeAudioCapture() error {
	if isAndroidTestAudioMock() {
		return nil
	}
	switch int(C.stAndroidStartMic()) {
	case 0:
		return nil
	case -2:
		return fmt.Errorf("brak uprawnień do mikrofonu w ustawieniach systemu Android")
	default:
		return fmt.Errorf("nie udało się uruchomić mikrofonu na urządzeniu Android")
	}
}

func PollNativeAudioCapturePeak() (float64, bool) {
	if isAndroidTestAudioMock() {
		return 0.35, true
	}
	var active C.int
	peak := float64(C.stAndroidPollPeak(&active))
	return peak, active != 0
}

func StopNativeAudioCapture() ([]byte, int, error) {
	if isAndroidTestAudioMock() {
		return make([]byte, 48000), 24000, nil
	}
	var byteLen C.size_t
	var sr C.int
	rawPtr := C.stAndroidStopMic(&byteLen, &sr)
	if rawPtr == nil || byteLen == 0 {
		return nil, int(sr), fmt.Errorf("brak zarejestrowanych próbek audio z mikrofonu")
	}
	defer C.free(unsafe.Pointer(rawPtr))
	return C.GoBytes(unsafe.Pointer(rawPtr), C.int(byteLen)), int(sr), nil
}
