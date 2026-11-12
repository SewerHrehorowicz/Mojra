package pl.mojra.app;

import android.Manifest;
import android.app.Activity;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.media.AudioFormat;
import android.media.AudioRecord;
import android.media.MediaRecorder;
import android.net.Uri;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.util.Base64;
import android.webkit.JavascriptInterface;
import android.webkit.PermissionRequest;
import android.webkit.ValueCallback;
import android.webkit.WebChromeClient;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.net.Socket;
import java.util.Arrays;
import java.util.Locale;

public class MainActivity extends Activity {
    private static Process serverProcess;
    private WebView webView;
    private ValueCallback<Uri[]> filePathCallback;
    private PermissionRequest pendingWebPermissionRequest;
    private static final int PORT = 18484;
    private static final int REQ_FILE_CHOOSER = 1001;
    private static final int REQ_RECORD_AUDIO = 1002;

    private final Object audioLock = new Object();
    private AudioRecord audioRecord;
    private Thread audioRecordThread;
    private volatile boolean isAudioCapturing = false;
    private volatile boolean audioPermissionPending = false;
    private volatile boolean audioPermissionDenied = false;
    private volatile float lastAudioPeak = 0.0f;
    private volatile int activeAudioSampleRate = 24000;
    private short[] audioPCMBuf = new short[0];
    private int audioPCMLen = 0;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        webView = new WebView(this);
        WebSettings ws = webView.getSettings();
        ws.setJavaScriptEnabled(true);
        ws.setDomStorageEnabled(true);
        ws.setAllowFileAccess(true);
        ws.setMediaPlaybackRequiresUserGesture(false);
        webView.addJavascriptInterface(new MojraAndroidBridge(), "mojraAndroid");
        webView.setWebViewClient(new WebViewClient());
        webView.setWebChromeClient(new WebChromeClient() {
            @Override
            public void onPermissionRequest(final PermissionRequest request) {
                for (String res : request.getResources()) {
                    if (PermissionRequest.RESOURCE_AUDIO_CAPTURE.equals(res)) {
                        if (checkSelfPermission(Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED) {
                            request.grant(new String[]{PermissionRequest.RESOURCE_AUDIO_CAPTURE});
                        } else {
                            pendingWebPermissionRequest = request;
                            requestPermissions(new String[]{Manifest.permission.RECORD_AUDIO}, REQ_RECORD_AUDIO);
                        }
                        return;
                    }
                }
                request.deny();
            }

            @Override
            public boolean onShowFileChooser(WebView wv, ValueCallback<Uri[]> filePathCb, FileChooserParams params) {
                if (filePathCallback != null) {
                    filePathCallback.onReceiveValue(null);
                }
                filePathCallback = filePathCb;
                try {
                    Intent intent = params.createIntent();
                    startActivityForResult(intent, REQ_FILE_CHOOSER);
                    return true;
                } catch (Exception e) {
                    filePathCallback = null;
                    return false;
                }
            }
        });
        setContentView(webView);

        new Thread(new Runnable() {
            @Override
            public void run() {
                startEmbeddedServer();
            }
        }).start();
    }

    private boolean startAndroidAudioRecordLocked() {
        stopAndroidAudioRecordLocked();
        int[] candidateRates = new int[]{24000, 16000, 44100, 48000};
        int[] sources = new int[]{MediaRecorder.AudioSource.MIC, MediaRecorder.AudioSource.DEFAULT};
        AudioRecord rec = null;
        int chosenRate = 24000;
        int chosenBufBytes = 0;

        outer:
        for (int src : sources) {
            for (int sr : candidateRates) {
                int minBuf = AudioRecord.getMinBufferSize(sr, AudioFormat.CHANNEL_IN_MONO, AudioFormat.ENCODING_PCM_16BIT);
                if (minBuf <= 0) {
                    continue;
                }
                int bufBytes = Math.max(minBuf * 2, (sr / 5) * 2);
                try {
                    AudioRecord candidate = new AudioRecord(src, sr, AudioFormat.CHANNEL_IN_MONO, AudioFormat.ENCODING_PCM_16BIT, bufBytes);
                    if (candidate.getState() == AudioRecord.STATE_INITIALIZED) {
                        candidate.startRecording();
                        if (candidate.getRecordingState() == AudioRecord.RECORDSTATE_RECORDING) {
                            rec = candidate;
                            chosenRate = sr;
                            chosenBufBytes = bufBytes;
                            break outer;
                        }
                    }
                    candidate.release();
                } catch (Exception ignored) {
                }
            }
        }
        if (rec == null) {
            return false;
        }
        activeAudioSampleRate = chosenRate;
        lastAudioPeak = 0.0f;
        audioPCMLen = 0;
        int minCap = chosenRate * 60;
        if (audioPCMBuf.length < minCap) {
            audioPCMBuf = new short[minCap];
        }
        audioRecord = rec;
        isAudioCapturing = true;
        final AudioRecord activeRec = rec;
        final int readSamples = Math.max(512, Math.min(2048, chosenBufBytes / 4));
        final int maxSamples = chosenRate * 1800;
        final int rateCap = chosenRate;
        audioRecordThread = new Thread(new Runnable() {
            @Override
            public void run() {
                short[] temp = new short[readSamples];
                while (isAudioCapturing) {
                    int n = activeRec.read(temp, 0, temp.length);
                    if (n <= 0) {
                        continue;
                    }
                    float maxAbs = 0.0f;
                    for (int i = 0; i < n; i++) {
                        float a = Math.abs((float) temp[i] / 32768.0f);
                        if (a > maxAbs) {
                            maxAbs = a;
                        }
                    }
                    float scaled = maxAbs * 1.8f;
                    if (scaled > 1.0f) {
                        scaled = 1.0f;
                    }
                    synchronized (audioLock) {
                        if (!isAudioCapturing || audioRecord != activeRec) {
                            break;
                        }
                        lastAudioPeak = scaled;
                        if (audioPCMLen + n <= maxSamples) {
                            if (audioPCMLen + n > audioPCMBuf.length) {
                                int newCap = Math.max(rateCap * 60, audioPCMBuf.length * 2);
                                while (newCap < audioPCMLen + n) {
                                    newCap *= 2;
                                }
                                audioPCMBuf = Arrays.copyOf(audioPCMBuf, newCap);
                            }
                            System.arraycopy(temp, 0, audioPCMBuf, audioPCMLen, n);
                            audioPCMLen += n;
                        }
                    }
                }
            }
        }, "MojraAudioRecord");
        audioRecordThread.start();
        return true;
    }

    private void stopAndroidAudioRecordLocked() {
        isAudioCapturing = false;
        AudioRecord rec = audioRecord;
        audioRecord = null;
        if (rec != null) {
            try {
                rec.stop();
            } catch (Exception ignored) {
            }
            try {
                rec.release();
            } catch (Exception ignored) {
            }
        }
    }

    private class MojraAndroidBridge {
        @JavascriptInterface
        public int startAudioCapture() {
            synchronized (audioLock) {
                audioPermissionDenied = false;
                if (checkSelfPermission(Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED) {
                    audioPermissionPending = true;
                    runOnUiThread(new Runnable() {
                        @Override
                        public void run() {
                            requestPermissions(new String[]{Manifest.permission.RECORD_AUDIO}, REQ_RECORD_AUDIO);
                        }
                    });
                    return 2;
                }
                audioPermissionPending = false;
                return startAndroidAudioRecordLocked() ? 1 : -1;
            }
        }

        @JavascriptInterface
        public String pollAudioPeak() {
            if (audioPermissionPending) {
                return "pending";
            }
            if (audioPermissionDenied) {
                return "denied";
            }
            if (!isAudioCapturing) {
                return "stopped";
            }
            return String.format(Locale.US, "%.3f", lastAudioPeak);
        }

        @JavascriptInterface
        public int getAudioSampleRate() {
            return activeAudioSampleRate;
        }

        @JavascriptInterface
        public String stopAudioCaptureBase64() {
            Thread t;
            synchronized (audioLock) {
                audioPermissionPending = false;
                t = audioRecordThread;
                audioRecordThread = null;
                stopAndroidAudioRecordLocked();
            }
            if (t != null) {
                try {
                    t.join(400);
                } catch (Exception ignored) {
                }
            }
            synchronized (audioLock) {
                int count = audioPCMLen;
                audioPCMLen = 0;
                lastAudioPeak = 0.0f;
                if (count <= 0) {
                    return "";
                }
                byte[] bytes = new byte[count * 2];
                for (int i = 0; i < count; i++) {
                    short v = audioPCMBuf[i];
                    bytes[i * 2] = (byte) (v & 0xff);
                    bytes[i * 2 + 1] = (byte) ((v >> 8) & 0xff);
                }
                return Base64.encodeToString(bytes, Base64.NO_WRAP);
            }
        }
    }

    @Override
    public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] grantResults) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults);
        if (requestCode == REQ_RECORD_AUDIO) {
            boolean granted = grantResults != null && grantResults.length > 0 && grantResults[0] == PackageManager.PERMISSION_GRANTED;
            if (pendingWebPermissionRequest != null) {
                if (granted) {
                    pendingWebPermissionRequest.grant(new String[]{PermissionRequest.RESOURCE_AUDIO_CAPTURE});
                } else {
                    pendingWebPermissionRequest.deny();
                }
                pendingWebPermissionRequest = null;
            }
            synchronized (audioLock) {
                if (audioPermissionPending) {
                    audioPermissionPending = false;
                    if (granted) {
                        if (!startAndroidAudioRecordLocked()) {
                            audioPermissionDenied = true;
                        }
                    } else {
                        audioPermissionDenied = true;
                    }
                }
            }
        }
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode == REQ_FILE_CHOOSER && filePathCallback != null) {
            Uri[] results = WebChromeClient.FileChooserParams.parseResult(resultCode, data);
            filePathCallback.onReceiveValue(results);
            filePathCallback = null;
        }
    }

    @Override
    public void onBackPressed() {
        if (webView != null && webView.canGoBack()) {
            webView.goBack();
            return;
        }
        super.onBackPressed();
    }

    private synchronized void startEmbeddedServer() {
        try {
            File filesDir = getFilesDir();
            File booksDir = new File(filesDir, "books");
            if (!booksDir.exists()) {
                booksDir.mkdirs();
            }
            String[] assetBooks = getAssets().list("books");
            if (assetBooks != null) {
                for (String name : assetBooks) {
                    File dst = new File(booksDir, name);
                    if (!dst.exists()) {
                        try (InputStream in = getAssets().open("books/" + name);
                             FileOutputStream out = new FileOutputStream(dst)) {
                            byte[] buf = new byte[16384];
                            int n;
                            while ((n = in.read(buf)) > 0) {
                                out.write(buf, 0, n);
                            }
                        }
                    }
                }
            }
            if (serverProcess == null) {
                String libDir = getApplicationInfo().nativeLibraryDir;
                File binFile = new File(libDir, "libmojra.so");
                ProcessBuilder pb = new ProcessBuilder(
                    binFile.getAbsolutePath(), "run", "--headless", "--port", String.valueOf(PORT)
                );
                pb.directory(filesDir);
                pb.environment().put("ST_NO_WINDOW", "1");
                pb.environment().put("MOJRA_ANDROID_APK", "1");
                pb.environment().put("MOJRA_DATA_DIR", filesDir.getAbsolutePath());
                pb.environment().put("HOME", filesDir.getAbsolutePath());
                serverProcess = pb.start();
            }

            for (int i = 0; i < 60; i++) {
                try (Socket s = new Socket("127.0.0.1", PORT)) {
                    break;
                } catch (Exception ignored) {
                    Thread.sleep(60);
                }
            }

            new Handler(Looper.getMainLooper()).post(new Runnable() {
                @Override
                public void run() {
                    if (webView != null) {
                        webView.loadUrl("http://127.0.0.1:" + PORT + "/");
                    }
                }
            });
        } catch (Exception e) {
            e.printStackTrace();
        }
    }

    @Override
    protected void onDestroy() {
        synchronized (audioLock) {
            stopAndroidAudioRecordLocked();
        }
        if (isFinishing() && serverProcess != null) {
            serverProcess.destroy();
            serverProcess = null;
        }
        super.onDestroy();
    }
}
