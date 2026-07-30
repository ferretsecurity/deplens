// A plain library is available to project/ build-definition code.
libraryDependencies += "org.example" % "build-utilities" % "1.3.0"
libraryDependencies ++= Seq("com.typesafe" % "config" % "1.4.3", "org.slf4j" % "slf4j-api" % "2.0.13")
libraryDependencies += "org.example" %% "cross-built-utility" % "1.0.0" % Test
