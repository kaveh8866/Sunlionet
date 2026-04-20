package com.sunlionet.agent

import android.app.Activity
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Bundle
import android.widget.TextView
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.camera.view.PreviewView
import androidx.core.content.ContextCompat
import com.google.mlkit.vision.barcode.BarcodeScanner
import com.google.mlkit.vision.barcode.BarcodeScannerOptions
import com.google.mlkit.vision.barcode.BarcodeScanning
import com.google.mlkit.vision.barcode.common.Barcode
import com.google.mlkit.vision.common.InputImage

class QrScanActivity : AppCompatActivity() {
    private lateinit var previewView: PreviewView
    private lateinit var textHint: TextView
    private var finished = false

    private val requestCameraPermission = registerForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        if (granted) {
            startCamera()
        } else {
            setResult(Activity.RESULT_CANCELED)
            finish()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_qr_scan)
        previewView = findViewById(R.id.previewView)
        textHint = findViewById(R.id.textQrHint)
        textHint.text = getString(R.string.scan_qr)

        val granted = ContextCompat.checkSelfPermission(this, android.Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED
        if (granted) {
            startCamera()
        } else {
            requestCameraPermission.launch(android.Manifest.permission.CAMERA)
        }
    }

    private fun startCamera() {
        val providerFuture = ProcessCameraProvider.getInstance(this)
        providerFuture.addListener(
            {
                val provider = providerFuture.get()
                val preview = Preview.Builder().build().also { it.setSurfaceProvider(previewView.surfaceProvider) }
                val scanner = BarcodeScanning.getClient(
                    BarcodeScannerOptions.Builder()
                        .setBarcodeFormats(Barcode.FORMAT_QR_CODE)
                        .build(),
                )
                val analysis = ImageAnalysis.Builder()
                    .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                    .build()

                analysis.setAnalyzer(
                    ContextCompat.getMainExecutor(this),
                ) { imageProxy ->
                    analyzeFrame(scanner, imageProxy)
                }

                runCatching {
                    provider.unbindAll()
                    provider.bindToLifecycle(this, CameraSelector.DEFAULT_BACK_CAMERA, preview, analysis)
                }.onFailure {
                    setResult(Activity.RESULT_CANCELED)
                    finish()
                }
            },
            ContextCompat.getMainExecutor(this),
        )
    }

    private fun analyzeFrame(scanner: BarcodeScanner, imageProxy: ImageProxy) {
        if (finished) {
            imageProxy.close()
            return
        }
        @androidx.camera.core.ExperimentalGetImage
        val mediaImage = imageProxy.image
        if (mediaImage == null) {
            imageProxy.close()
            return
        }
        val image = InputImage.fromMediaImage(mediaImage, imageProxy.imageInfo.rotationDegrees)
        scanner.process(image)
            .addOnSuccessListener { barcodes ->
                val text = barcodes.firstOrNull()?.rawValue?.trim().orEmpty()
                if (text.isNotBlank()) {
                    finished = true
                    val data = Intent().putExtra(EXTRA_QR_TEXT, text)
                    setResult(Activity.RESULT_OK, data)
                    finish()
                }
            }
            .addOnCompleteListener {
                imageProxy.close()
            }
    }

    companion object {
        const val EXTRA_QR_TEXT = "qr_text"
    }
}
