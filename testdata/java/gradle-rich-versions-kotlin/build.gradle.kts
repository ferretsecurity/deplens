dependencies {
  implementation("org.example:core") {
    version {
      strictly("[1.0,2.0[")
      prefer("vendor:preview")
      reject("org.example:phantom:1.6")
    }
  }
  implementation("org.example:dynamic") {
    version {
      require(selectedVersion)
    }
  }
}
