package org.calltoverify.receiver.ui

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.core.content.ContextCompat
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.calltoverify.receiver.ReceiverApp
import org.calltoverify.receiver.ReceiverRepository
import org.calltoverify.receiver.data.Pairing
import org.calltoverify.receiver.data.PairingParser
import org.calltoverify.receiver.net.CtvClient
import org.calltoverify.receiver.service.ReceiverService

/**
 * The single Activity. Hosts the Compose UI and brokers the runtime permissions
 * and the pairing/registration flow between the UI and the [ReceiverRepository].
 *
 * Two top-level states, driven by [ReceiverRepository.paired]:
 *  - not paired -> [PairingScreen] (scan a QR, parse, store, register)
 *  - paired     -> [ConnectedScreen] (status + recent inbound log)
 */
class MainActivity : ComponentActivity() {

    private val repository: ReceiverRepository
        get() = ReceiverApp.repository

    /** Camera permission request, resolved into [cameraGranted] for the UI. */
    private val cameraGranted = androidx.compose.runtime.mutableStateOf(false)
    private val requestCamera =
        registerForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
            cameraGranted.value = granted
        }

    /**
     * The runtime permissions the receiver channels need. POST_NOTIFICATIONS is
     * only requested on API 33+. We request these together once paired so the
     * service notification shows and the call/SMS channels can function.
     */
    private val requestRuntimePerms =
        registerForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { /* best effort */ }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        cameraGranted.value = hasCameraPermission()

        setContent {
            CallToVerifyTheme {
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = MaterialTheme.colorScheme.background,
                ) {
                    val paired by repository.paired.collectAsState()

                    if (paired) {
                        ConnectedScreen(
                            repository = repository,
                            onUnpair = ::unpair,
                        )
                    } else {
                        PairingScreen(
                            cameraGranted = cameraGranted.value,
                            onRequestCamera = { requestCamera.launch(Manifest.permission.CAMERA) },
                            onScanned = ::onQrScanned,
                        )
                    }
                }
            }
        }
    }

    override fun onResume() {
        super.onResume()
        // Permission may have been toggled in system settings while we were away.
        cameraGranted.value = hasCameraPermission()
        // If we're paired, make sure the service is running (e.g. after the user
        // returns from Settings, or the process was recreated).
        if (repository.paired.value) {
            ReceiverService.start(this)
        }
    }

    private fun hasCameraPermission(): Boolean =
        ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA) ==
            PackageManager.PERMISSION_GRANTED

    /**
     * Handles a decoded QR string: parse -> persist -> register. On a register
     * failure we keep the stored pairing (the foreground service will retry), but
     * report the outcome to the UI via [PairingScreen]'s status callback.
     */
    private fun onQrScanned(raw: String, onStatus: (PairingOutcome) -> Unit) {
        when (val result = PairingParser.parse(raw)) {
            is PairingParser.Result.Error -> onStatus(PairingOutcome.Invalid(result.message))
            is PairingParser.Result.Ok -> {
                onStatus(PairingOutcome.Registering)
                pairAndRegister(result.pairing, onStatus)
            }
        }
    }

    private fun pairAndRegister(pairing: Pairing, onStatus: (PairingOutcome) -> Unit) {
        repository.savePairing(pairing)
        requestRuntimePermissions()
        lifecycleScope.launch {
            val outcome = withContext(Dispatchers.IO) {
                CtvClient(pairing).register()
            }
            when (outcome) {
                is CtvClient.Outcome.Ok -> {
                    repository.updateNumbers(outcome.value.numbers)
                    repository.setOnline(true)
                    // Start the heartbeat/retry service now that we're paired.
                    ReceiverService.start(this@MainActivity)
                    onStatus(PairingOutcome.Success)
                    // paired flow flips automatically via the repository StateFlow.
                }
                is CtvClient.Outcome.HttpError -> {
                    // Keep the pairing; the service will keep trying. Surface the code.
                    ReceiverService.start(this@MainActivity)
                    onStatus(PairingOutcome.RegisterFailed("Server returned ${outcome.code}"))
                }
                is CtvClient.Outcome.NetworkError -> {
                    ReceiverService.start(this@MainActivity)
                    onStatus(PairingOutcome.RegisterFailed("Network error; will retry"))
                }
            }
        }
    }

    private fun requestRuntimePermissions() {
        val perms = buildList {
            add(Manifest.permission.READ_PHONE_STATE)
            add(Manifest.permission.READ_CALL_LOG)
            add(Manifest.permission.ANSWER_PHONE_CALLS)
            add(Manifest.permission.RECEIVE_SMS)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                add(Manifest.permission.POST_NOTIFICATIONS)
            }
        }.toTypedArray()
        requestRuntimePerms.launch(perms)
    }

    private fun unpair() {
        ReceiverService.stop(this)
        repository.unpair()
    }
}
