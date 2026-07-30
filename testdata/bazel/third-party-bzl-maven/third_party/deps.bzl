load("@rules_jvm_external//:defs.bzl", "maven_install")

def third_party_dependencies():
    maven_install(
        name = "maven",
        artifacts = [
            "com.google.guava:guava:33.3.1-jre",
            "org.junit.jupiter:junit-jupiter-api:5.11.3",
        ],
        repositories = ["https://repo1.maven.org/maven2"],
        excluded_artifacts = ["com.google.code.findbugs:jsr305"],
        strict_visibility = True,
    )
