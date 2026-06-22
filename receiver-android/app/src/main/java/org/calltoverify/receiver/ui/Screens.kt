package org.calltoverify.receiver.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.QrCodeScanner
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import org.calltoverify.receiver.R
import org.calltoverify.receiver.ReceiverRepository
import org.calltoverify.receiver.data.InboundLogEntry
import org.calltoverify.receiver.data.InboundType
import org.calltoverify.receiver.data.ProvisionedNumber
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/** Result of attempting to pair, surfaced to [PairingScreen] for status text. */
sealed interface PairingOutcome {
    data class Invalid(val message: String) : PairingOutcome
    data object Registering : PairingOutcome
    data object Success : PairingOutcome
    data class RegisterFailed(val message: String) : PairingOutcome
}

// ---------------------------------------------------------------------------
// Not-paired: scan a QR to pair.
// ---------------------------------------------------------------------------

@Composable
fun PairingScreen(
    cameraGranted: Boolean,
    onRequestCamera: () -> Unit,
    onScanned: (raw: String, onStatus: (PairingOutcome) -> Unit) -> Unit,
) {
    var scanning by remember { mutableStateOf(false) }
    var status by remember { mutableStateOf<PairingOutcome?>(null) }

    if (scanning && cameraGranted) {
        Box(modifier = Modifier.fillMaxSize()) {
            QrScanner(
                modifier = Modifier.fillMaxSize(),
                onResult = { raw ->
                    scanning = false
                    onScanned(raw) { outcome -> status = outcome }
                },
            )
            // Simple framing overlay + cancel.
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(24.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                Text(
                    text = stringResource(R.string.scan_hint),
                    color = Color.White,
                    style = MaterialTheme.typography.titleMedium,
                )
            }
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(32.dp),
                contentAlignment = Alignment.BottomCenter,
            ) {
                OutlinedButton(onClick = { scanning = false }) {
                    Text(stringResource(R.string.cancel))
                }
            }
        }
        return
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(28.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center,
    ) {
        BrandMark()
        Spacer(Modifier.height(20.dp))
        Text(
            text = stringResource(R.string.app_name),
            style = MaterialTheme.typography.headlineMedium,
            fontWeight = FontWeight.Bold,
        )
        Spacer(Modifier.height(8.dp))
        Text(
            text = stringResource(R.string.pairing_subtitle),
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Spacer(Modifier.height(36.dp))

        Button(
            onClick = {
                status = null
                if (cameraGranted) {
                    scanning = true
                } else {
                    onRequestCamera()
                }
            },
            modifier = Modifier.fillMaxWidth(),
        ) {
            Icon(Icons.Filled.QrCodeScanner, contentDescription = null)
            Spacer(Modifier.width(10.dp))
            Text(stringResource(R.string.scan_to_pair))
        }

        if (!cameraGranted) {
            Spacer(Modifier.height(12.dp))
            Text(
                text = stringResource(R.string.camera_needed),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }

        status?.let { StatusLine(it) }
    }
}

@Composable
private fun StatusLine(outcome: PairingOutcome) {
    Spacer(Modifier.height(20.dp))
    val (text, color) = when (outcome) {
        is PairingOutcome.Invalid ->
            outcome.message to MaterialTheme.colorScheme.error
        PairingOutcome.Registering ->
            stringResource(R.string.registering) to MaterialTheme.colorScheme.onSurfaceVariant
        PairingOutcome.Success ->
            stringResource(R.string.paired_ok) to OnlineGreen
        is PairingOutcome.RegisterFailed ->
            outcome.message to OfflineAmber
    }
    Text(text = text, color = color, style = MaterialTheme.typography.bodyMedium)
}

@Composable
private fun BrandMark() {
    Box(
        modifier = Modifier
            .size(84.dp)
            .clip(RoundedCornerShape(22.dp))
            .background(MaterialTheme.colorScheme.primary),
        contentAlignment = Alignment.Center,
    ) {
        Icon(
            imageVector = Icons.Filled.QrCodeScanner,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onPrimary,
            modifier = Modifier.size(44.dp),
        )
    }
}

// ---------------------------------------------------------------------------
// Paired: connected status + recent inbound log.
// ---------------------------------------------------------------------------

@Composable
fun ConnectedScreen(
    repository: ReceiverRepository,
    onUnpair: () -> Unit,
) {
    val pairing by repository.pairing.collectAsState()
    val online by repository.online.collectAsState()
    val numbers by repository.numbers.collectAsState()
    val log by repository.log.collectAsState()
    val pending by repository.pendingCount.collectAsState()

    LazyColumn(
        modifier = Modifier
            .fillMaxSize()
            .padding(horizontal = 20.dp),
        contentPadding = PaddingValues(vertical = 24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        item {
            Row(verticalAlignment = Alignment.CenterVertically) {
                StatusDot(online = online)
                Spacer(Modifier.width(10.dp))
                Text(
                    text = stringResource(
                        if (online) R.string.status_online else R.string.status_offline,
                    ),
                    style = MaterialTheme.typography.headlineSmall,
                    fontWeight = FontWeight.Bold,
                )
            }
        }

        item {
            InfoCard {
                InfoRow(stringResource(R.string.label_endpoint), pairing?.endpoint ?: "—")
                InfoRow(stringResource(R.string.label_device_id), pairing?.deviceId ?: "—")
                if (pending > 0) {
                    InfoRow(
                        stringResource(R.string.label_pending),
                        pending.toString(),
                    )
                }
            }
        }

        item {
            SectionHeader(stringResource(R.string.section_numbers))
        }
        if (numbers.isEmpty()) {
            item { EmptyHint(stringResource(R.string.no_numbers)) }
        } else {
            items(numbers) { number -> NumberRow(number) }
        }

        item {
            SectionHeader(stringResource(R.string.section_recent))
        }
        if (log.isEmpty()) {
            item { EmptyHint(stringResource(R.string.no_events)) }
        } else {
            items(log) { entry -> LogRow(entry) }
        }

        item {
            Spacer(Modifier.height(8.dp))
            OutlinedButton(
                onClick = onUnpair,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(stringResource(R.string.unpair))
            }
        }
    }
}

@Composable
private fun StatusDot(online: Boolean) {
    Box(
        modifier = Modifier
            .size(14.dp)
            .clip(CircleShape)
            .background(if (online) OnlineGreen else OfflineAmber),
    )
}

@Composable
private fun InfoCard(content: @Composable () -> Unit) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surfaceVariant,
        ),
        shape = RoundedCornerShape(16.dp),
    ) {
        Column(
            modifier = Modifier.padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            content()
        }
    }
}

@Composable
private fun InfoRow(label: String, value: String) {
    Column {
        Text(
            text = label,
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Text(
            text = value,
            style = MaterialTheme.typography.bodyMedium,
            fontFamily = FontFamily.Monospace,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
        )
    }
}

@Composable
private fun SectionHeader(text: String) {
    Text(
        text = text,
        style = MaterialTheme.typography.titleSmall,
        fontWeight = FontWeight.SemiBold,
        color = MaterialTheme.colorScheme.primary,
    )
}

@Composable
private fun EmptyHint(text: String) {
    Text(
        text = text,
        style = MaterialTheme.typography.bodySmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
    )
}

@Composable
private fun NumberRow(number: ProvisionedNumber) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface,
        ),
        shape = RoundedCornerShape(12.dp),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(14.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = number.msisdn.ifBlank { "—" },
                    style = MaterialTheme.typography.bodyLarge,
                    fontFamily = FontFamily.Monospace,
                )
                if (number.channels.isNotEmpty()) {
                    Text(
                        text = number.channels.joinToString(" · "),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            val statusColor =
                if (number.status == "active") OnlineGreen else MaterialTheme.colorScheme.onSurfaceVariant
            Text(
                text = number.status.ifBlank { "—" },
                style = MaterialTheme.typography.labelMedium,
                color = statusColor,
            )
        }
    }
}

private val TIME_FORMAT = SimpleDateFormat("HH:mm:ss", Locale.US)

@Composable
private fun LogRow(entry: InboundLogEntry) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = TIME_FORMAT.format(Date(entry.timestampMillis)),
            style = MaterialTheme.typography.labelSmall,
            fontFamily = FontFamily.Monospace,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.width(64.dp),
        )
        val typeLabel = when (entry.type) {
            InboundType.SMS -> stringResource(R.string.type_sms)
            InboundType.CALL -> stringResource(R.string.type_call)
        }
        Text(
            text = typeLabel,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.primary,
            modifier = Modifier.width(44.dp),
        )
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = entry.sender.ifBlank { "—" },
                style = MaterialTheme.typography.bodyMedium,
                fontFamily = FontFamily.Monospace,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Text(
                text = entry.summary,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}
