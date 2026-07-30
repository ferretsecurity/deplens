import sbt._

object Dependencies {
  val excluded = ("com.example" % "exclude-kit" % "1.0.0").exclude("org.slf4j", "slf4j-api")
  val documented = ("com.example" % "docs-kit" % "2.0.0").withSources().withJavadoc().extra("build" -> "fixture").force()
  val fullyExcluded = ("com.example" % "multi-exclude-kit" % "3.0.0").excludeAll(
    ExclusionRule("org.slf4j", "slf4j-api"),
    ExclusionRule("commons-logging", "commons-logging")
  )
}
