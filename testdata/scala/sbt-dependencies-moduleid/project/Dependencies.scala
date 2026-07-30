import sbt._

object Dependencies {
  val positional = ModuleID("com.example", "positional-artifact", "1.0.0")
  val named = ModuleID(organization = "com.example", name = "named-artifact", revision = "2.0.0")
}
