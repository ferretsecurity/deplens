import sbt._

object Dependencies {
  lazy val commonVersion = "1.0.0"
  val core: Seq[ModuleID] = Seq("com.example" % "range-kit" % "[1.0,)", ModuleID("com.example", "direct-kit", commonVersion))
  val all = core ++ List[ModuleID]("com.example" %% "latest-kit" % "latest.integration", "com.example" % "plus-kit" % "2.9.+", "com.example" % "mapped-kit" % "3.0.0" % "test->compile")
}
