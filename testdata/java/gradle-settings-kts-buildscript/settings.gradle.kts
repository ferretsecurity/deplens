buildscript {
    repositories {
        maven(url = "https://repo.example.test/plugins")
        mavenCentral()
    }
    dependencies {
        classpath("com.example:legacy-settings-plugin:2.5.0")
    }
}

rootProject.name = "settings-buildscript-kts"
