pluginManagement {
    plugins {
        id("com.example.hello") version providers.gradleProperty("helloPluginVersion").get()
    }
    repositories { gradlePluginPortal() }
}

rootProject.name = "dynamic-plugin-version-kts"
