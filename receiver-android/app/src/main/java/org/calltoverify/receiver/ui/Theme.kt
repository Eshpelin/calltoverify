package org.calltoverify.receiver.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

/**
 * The CallToVerify brand palette: an indigo accent (#4F46E5) on near-neutral
 * surfaces, with a green "online" and an amber "offline" signal used by the
 * status UI.
 */
private val Indigo = Color(0xFF4F46E5)
private val IndigoDark = Color(0xFF6366F1)
private val OnIndigo = Color(0xFFFFFFFF)
private val IndigoContainerLight = Color(0xFFE0E0FF)
private val IndigoContainerDark = Color(0xFF2E2A6B)

internal val OnlineGreen = Color(0xFF16A34A)
internal val OfflineAmber = Color(0xFFD97706)

private val LightColors = lightColorScheme(
    primary = Indigo,
    onPrimary = OnIndigo,
    primaryContainer = IndigoContainerLight,
    onPrimaryContainer = Color(0xFF1E1B4B),
    secondary = Indigo,
    onSecondary = OnIndigo,
    background = Color(0xFFFBFBFE),
    onBackground = Color(0xFF1B1B1F),
    surface = Color(0xFFFFFFFF),
    onSurface = Color(0xFF1B1B1F),
    surfaceVariant = Color(0xFFEDEDF4),
    onSurfaceVariant = Color(0xFF46464F),
)

private val DarkColors = darkColorScheme(
    primary = IndigoDark,
    onPrimary = Color(0xFF111133),
    primaryContainer = IndigoContainerDark,
    onPrimaryContainer = Color(0xFFE0E0FF),
    secondary = IndigoDark,
    onSecondary = Color(0xFF111133),
    background = Color(0xFF121218),
    onBackground = Color(0xFFE4E1E9),
    surface = Color(0xFF1B1B22),
    onSurface = Color(0xFFE4E1E9),
    surfaceVariant = Color(0xFF46464F),
    onSurfaceVariant = Color(0xFFC7C5D0),
)

@Composable
fun CallToVerifyTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit,
) {
    MaterialTheme(
        colorScheme = if (darkTheme) DarkColors else LightColors,
        typography = Typography(),
        content = content,
    )
}
