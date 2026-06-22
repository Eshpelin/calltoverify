// Root build script.
//
// Plugins are declared here with `apply false` so their versions are resolved
// once for the whole build; the :app module then applies them without repeating
// the version. This is the standard Gradle "plugins block in root, apply in
// module" pattern.
//
// We target Kotlin 1.9.24, where the Jetpack Compose compiler is configured via
// the Android `composeOptions { kotlinCompilerExtensionVersion = ... }` block in
// the module (see app/build.gradle.kts), so no separate Compose Gradle plugin is
// applied here.

plugins {
    id("com.android.application") version "8.5.2" apply false
    id("org.jetbrains.kotlin.android") version "1.9.24" apply false
}
