import mill._, scalalib._

object main extends ScalaModule {
  def scalaVersion = "3.3.1"
  def ivyDeps = Agg(ivy"org.slf4j:slf4j-api:2.0.13", ivy"com.fasterxml.jackson.core:jackson-databind:2.17.2")
}
