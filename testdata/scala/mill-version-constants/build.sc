import mill._, scalalib._

object versions {
  val plugin = "0.13.2"
  val core = "3.4.5"
}

object app extends ScalaModule {
  def scalacPluginIvyDeps = Agg(ivy"tools:::compiler-plugin:${versions.plugin}")
  def ivyDeps = Agg(
    ivy"demo:core:${versions.core}",
    ivy"demo:runtime:${scalaVersion()}"
  )
}
