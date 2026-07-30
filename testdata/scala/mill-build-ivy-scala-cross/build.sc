import mill._, scalalib._

object main extends ScalaModule {
  def scalaVersion = "2.12.14"
  def ivyDeps = Agg(ivy"com.lihaoyi::os-lib:0.10.2", ivy"org.scalamacros:::paradise:2.1.1")
}
