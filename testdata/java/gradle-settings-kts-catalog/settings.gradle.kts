dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories { mavenCentral(); google() }
    versionCatalogs {
        create("publishedLibs") { from("com.example:build-catalog:1.2.3") }
        create("testLibs") {
            from(files("gradle/test.versions.toml"))
            version("groovy", "3.0.22")
            library("groovy-json", "org.codehaus.groovy", "groovy-json").versionRef("groovy")
            plugin("versions", "com.github.ben-manes.versions").version("0.52.0")
            bundle("groovy", listOf("groovy-core", "groovy-json"))
        }
    }
}

rootProject.name = "published-catalog-kts"
