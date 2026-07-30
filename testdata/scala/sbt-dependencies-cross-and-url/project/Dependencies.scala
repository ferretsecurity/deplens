import sbt._

object Dependencies {
  val scala2LibraryOnScala3 = ("org.typelevel" % "cats-core" % "2.10.0").cross(CrossVersion.for3Use2_13)
  val downloadedJar = ("slinky" % "slinky" % "2.1").from("https://downloads.example.test/slinky-2.1.jar")
}
