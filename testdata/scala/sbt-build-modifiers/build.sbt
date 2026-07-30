libraryDependencies += ("org.testng" % "testng" % "7.10.2").classifier("jdk15")
libraryDependencies += ("org.apache.felix" % "org.apache.felix.framework" % "1.8.0").intransitive()
libraryDependencies += ("com.example" % "without-logging" % "2.0.0").exclude("org.slf4j", "slf4j-api")
