Test / libraryDependencies += "org.scalatest" %% "scalatest" % "3.2.18"
Compile / libraryDependencies ++= Seq("com.example" % "compile-kit" % "1.0.0", "com.example" % "support-kit" % "1.1.0")
libraryDependencies += "com.example" % "mapped-test-kit" % "2.0.0" % "test->compile"
libraryDependencies += "com.example" % "integration-kit" % "3.0.0" % "it,test"
