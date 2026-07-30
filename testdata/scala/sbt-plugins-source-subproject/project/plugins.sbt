lazy val root = (project in file(".")).dependsOn(pluginSubproject)
lazy val pluginSubproject = ProjectRef(uri("https://example.test/sbt/multi-plugin.git#v1.0.0"), "plugin")
