dependencyResolutionManagement {
    repositories {
        maven {
            name = "internal"
            url = uri("https://repo.example.test/maven")
            content { includeGroupByRegex("com\\.example(\\..*)?") }
        }
        ivy {
            url = uri("https://repo.example.test/ivy")
            metadataSources { artifact() }
        }
        mavenCentral()
    }
}

rootProject.name = "repository-content-kts"
