import sbt._

object Dependencies {
  val testOnly = "org.scalacheck" %% "scalacheck" % "1.18.0" % "test"
  val integration = "com.example" % "integration-api" % "1.0.0" % "it,test"
  val providedApi = "jakarta.servlet" % "jakarta.servlet-api" % "6.1.0" % Provided
}
