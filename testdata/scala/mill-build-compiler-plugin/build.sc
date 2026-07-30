import mill._, scalalib._

object main extends ScalaModule {
  def scalaVersion = "2.12.14"
  def compileIvyDeps = Agg(ivy"org.scala-lang:scala-reflect:2.12.14")
  def scalacPluginIvyDeps = Agg(ivy"org.scalamacros:::paradise:2.1.1")
}
