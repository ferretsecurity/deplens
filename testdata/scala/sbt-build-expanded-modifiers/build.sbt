libraryDependencies += ("org.apache.felix" % "org.apache.felix.framework" % "1.8.0").notTransitive()
libraryDependencies += ("com.example" % "fully-excluded-kit" % "2.0.0").excludeAll(ExclusionRule("org.slf4j", "slf4j-api"))
libraryDependencies += ("org.lwjgl.lwjgl" % "lwjgl-platform" % "2.9.3").classifier("natives-windows").classifier("natives-linux").classifier("natives-osx")
