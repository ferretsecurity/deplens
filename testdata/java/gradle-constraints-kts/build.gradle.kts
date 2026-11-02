dependencies {
  implementation("org.example:core:1.2")
  constraints {
    implementation("org.example:core:1.2")
    api(platform("org.example:platform:2.0"))
    compileOnly(
      group = "org.example",
      name = "compile",
      version = "3.0",
    )
  }
  testImplementation("org.example:test-kit:4.0")
}
