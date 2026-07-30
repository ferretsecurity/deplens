lazy val root = (project in file(".")).dependsOn(assemblyPlugin)
lazy val assemblyPlugin = RootProject(uri("git://github.com/sbt/sbt-assembly#v2.3.1"))
