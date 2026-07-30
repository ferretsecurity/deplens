sourceControl {
    gitRepository(uri("https://github.com/example/utilities.git")) {
        producesModule("com.example:utilities")
        producesModule("com.example:utilities-test-fixtures")
    }
}

rootProject.name = "source-controlled-dependencies-kts"
