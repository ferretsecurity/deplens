pluginManagement {
    plugins {
        id("com.gradleup.shadow") version "8.3.4"
        id("com.diffplug.spotless") version "7.0.2"
    }
    repositories { gradlePluginPortal() }
}

rootProject.name = "settings-kts-fixture"
include("api", "service:worker")
