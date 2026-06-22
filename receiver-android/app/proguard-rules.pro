# ProGuard / R8 rules for the receiver app.
#
# Release builds ship un-minified by default (see app/build.gradle.kts), so these
# rules are mostly a safety net for anyone who flips `isMinifyEnabled = true`.

# ML Kit barcode scanning loads model code reflectively; keep it intact.
-keep class com.google.mlkit.** { *; }
-keep class com.google.android.gms.** { *; }

# OkHttp ships optional Conscrypt/BouncyCastle hooks behind reflection.
-dontwarn okhttp3.internal.platform.**
-dontwarn org.conscrypt.**
-dontwarn org.bouncycastle.**
-dontwarn org.openjsse.**
