ThisBuild / scalaVersion := "2.13.12"

libraryDependencies += "org.scala-stm" %% "scala-stm" % "0.11.0"
libraryDependencies ++= Seq("org.typelevel" %% "cats-core" % "2.10.0", "com.typesafe" % "config" % "1.4.3")
