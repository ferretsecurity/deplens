import sbt._

object Dependencies {
  lazy val akkaVersion = "2.6.21"
  lazy val testVersion = "3.2.18"

  val akkaActor = "com.typesafe.akka" %% "akka-actor" % akkaVersion
  val scalaTest = "org.scalatest" %% "scalatest" % testVersion
  val backendDeps = Seq(akkaActor, scalaTest % Test)
}
