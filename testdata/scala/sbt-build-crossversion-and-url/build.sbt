ThisBuild / scalaVersion := "3.3.1"

libraryDependencies += (("org.typelevel" % "cats-core" % "2.10.0") cross CrossVersion.for3Use2_13)
libraryDependencies += "slinky" % "slinky" % "2.1" from "https://downloads.example.test/slinky-2.1.jar"
