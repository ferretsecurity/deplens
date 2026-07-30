import sbt._

object Dependencies {
  val testNg = ("org.testng" % "testng" % "7.10.2").classifier("jdk15")
  val noTransitively = ("org.apache.felix" % "org.apache.felix.framework" % "1.8.0").notTransitive()
  val withoutLogging = ("com.example" % "without-logging" % "2.0.0").excludeAll(ExclusionRule("org.slf4j", "slf4j-api"))
}
