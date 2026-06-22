package org.calltoverify.receiver.ui

import android.annotation.SuppressLint
import android.util.Log
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.viewinterop.AndroidView
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.LocalLifecycleOwner
import com.google.mlkit.vision.barcode.BarcodeScanning
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.common.InputImage
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean

/**
 * A full-bleed CameraX preview that scans for QR codes with ML Kit and reports the
 * first decoded value via [onResult].
 *
 * The camera is bound to the composition's lifecycle and torn down automatically.
 * [onResult] fires at most once per mount (guarded by an [AtomicBoolean]) so the
 * caller can navigate away without seeing duplicate scans.
 *
 * The caller is responsible for having obtained the CAMERA permission before this
 * is shown.
 */
@Composable
fun QrScanner(
    modifier: Modifier = Modifier,
    onResult: (String) -> Unit,
) {
    val lifecycleOwner = LocalLifecycleOwner.current
    val analysisExecutor = remember { Executors.newSingleThreadExecutor() }
    val delivered = remember { AtomicBoolean(false) }

    AndroidView(
        modifier = modifier.fillMaxSize(),
        factory = { ctx ->
            val previewView = PreviewView(ctx).apply {
                scaleType = PreviewView.ScaleType.FILL_CENTER
            }
            val providerFuture = ProcessCameraProvider.getInstance(ctx)
            providerFuture.addListener({
                val cameraProvider = providerFuture.get()

                val preview = Preview.Builder().build().also {
                    it.setSurfaceProvider(previewView.surfaceProvider)
                }

                val analysis = ImageAnalysis.Builder()
                    .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                    .build()
                    .also { it.setAnalyzer(analysisExecutor, QrAnalyzer(delivered, onResult)) }

                try {
                    cameraProvider.unbindAll()
                    cameraProvider.bindToLifecycle(
                        lifecycleOwner,
                        CameraSelector.DEFAULT_BACK_CAMERA,
                        preview,
                        analysis,
                    )
                } catch (t: Throwable) {
                    Log.e("QrScanner", "Failed to bind camera use cases", t)
                }
            }, ContextCompat.getMainExecutor(ctx))

            previewView
        },
    )

    DisposableEffect(Unit) {
        onDispose { analysisExecutor.shutdown() }
    }
}

/**
 * ML Kit analyzer that decodes QR codes off the camera stream and forwards the
 * first non-empty value exactly once.
 */
private class QrAnalyzer(
    private val delivered: AtomicBoolean,
    private val onResult: (String) -> Unit,
) : ImageAnalysis.Analyzer {

    private val scanner = BarcodeScanning.getClient()

    @SuppressLint("UnsafeOptInUsageError")
    override fun analyze(imageProxy: ImageProxy) {
        if (delivered.get()) {
            imageProxy.close()
            return
        }
        val mediaImage = imageProxy.image
        if (mediaImage == null) {
            imageProxy.close()
            return
        }
        val input = InputImage.fromMediaImage(mediaImage, imageProxy.imageInfo.rotationDegrees)
        scanner.process(input)
            .addOnSuccessListener { barcodes ->
                val value = barcodes.firstOrNull { it.format == Barcode.FORMAT_QR_CODE }?.rawValue
                if (!value.isNullOrBlank() && delivered.compareAndSet(false, true)) {
                    onResult(value)
                }
            }
            .addOnFailureListener { e ->
                Log.w("QrScanner", "Barcode scan failed", e)
            }
            .addOnCompleteListener {
                imageProxy.close()
            }
    }
}
