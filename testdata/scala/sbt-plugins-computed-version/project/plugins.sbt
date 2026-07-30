val assemblyVersion = sys.props.getOrElse("plugin.version", "2.3.1")
addSbtPlugin("com.eed3si9n" % "sbt-assembly" % assemblyVersion)
