// Module build script for the receiver app.

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "org.calltoverify.receiver"
    compileSdk = 34

    defaultConfig {
        applicationId = "org.calltoverify.receiver"
        minSdk = 24
        targetSdk = 34
        versionCode = 1
        versionName = "0.1.0"

        // No instrumentation tests ship yet, but wire the runner so `./gradlew
        // connectedCheck` works if tests are added later.
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    signingConfigs {
        create("release") {
            // Supplied by CI (or a maintainer) via env. Without them the release
            // build is left unsigned rather than falling back to the public debug
            // key — but it is still non-debuggable, which is the property that
            // matters for protecting the on-device device_secret.
            System.getenv("CTV_KEYSTORE_FILE")?.let { ks ->
                storeFile = file(ks)
                storePassword = System.getenv("CTV_KEYSTORE_PASSWORD")
                keyAlias = System.getenv("CTV_KEY_ALIAS")
                keyPassword = System.getenv("CTV_KEY_PASSWORD")
            }
        }
    }

    buildTypes {
        release {
            // Sideloaded, open-source build: keep it un-minified so the APK is
            // easy to inspect and reproduce. Flip these on for size if desired.
            isMinifyEnabled = false
            // Never ship a debuggable receiver: debuggable=true lets anyone with
            // ADB run-as the app and recover the device_secret.
            isDebuggable = false
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
            if (System.getenv("CTV_KEYSTORE_FILE") != null) {
                signingConfig = signingConfigs.getByName("release")
            }
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    buildFeatures {
        compose = true
        buildConfig = true // exposes BuildConfig.DEBUG to gate the test-only pairing intent
    }

    composeOptions {
        // Compose compiler extension compatible with Kotlin 1.9.24.
        kotlinCompilerExtensionVersion = "1.5.14"
    }

    packaging {
        resources {
            excludes += "/META-INF/{AL2.0,LGPL2.1}"
        }
    }
}

dependencies {
    // --- Kotlin / coroutines ---
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.8.1")

    // --- AndroidX core + lifecycle ---
    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.activity:activity-compose:1.9.1")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.8.4")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.4")
    implementation("androidx.lifecycle:lifecycle-service:2.8.4")

    // --- Jetpack Compose (UI) ---
    implementation(platform("androidx.compose:compose-bom:2024.06.00"))
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-extended")
    debugImplementation("androidx.compose.ui:ui-tooling")

    // --- Encrypted persistence for endpoint/device_id/device_secret ---
    implementation("androidx.security:security-crypto:1.1.0-alpha06")

    // --- HTTP (signed device requests) ---
    implementation("com.squareup.okhttp3:okhttp:4.12.0")

    // --- Camera + barcode scanning for QR pairing ---
    implementation("androidx.camera:camera-core:1.3.4")
    implementation("androidx.camera:camera-camera2:1.3.4")
    implementation("androidx.camera:camera-lifecycle:1.3.4")
    implementation("androidx.camera:camera-view:1.3.4")
    implementation("com.google.mlkit:barcode-scanning:17.3.0")

    // --- Tests ---
    testImplementation("junit:junit:4.13.2")
}
