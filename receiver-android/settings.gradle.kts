// Gradle settings for the CallToVerify Android receiver.
//
// Declares the plugin and dependency repositories once for the whole build, and
// wires in the single :app module. Keeping repositories in settings (rather than
// per-project) is the modern, recommended layout for AndroidX/Gradle.

pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

dependencyResolutionManagement {
    // Fail the build if a module declares its own repositories; everything must
    // come from the central list below. This keeps dependency sources auditable
    // for an open-source project.
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
    }
}

rootProject.name = "CallToVerify Receiver"
include(":app")
