pluginManagement {
    includeBuild("build-logic") {
        dependencySubstitution {
            substitute(module("com.example:conventions")).using(project(":"))
        }
    }
}

includeBuild("../shared-platform") { name = "shared-platform" }
rootProject.name = "composite-settings-kts"
