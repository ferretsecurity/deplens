import mill._, scalalib._
import $ivy.`build.tools::format-plugin::1.0.0`
import $ivy.`build.tools::lint-plugin::2.0.0`

object app extends ScalaModule {
  def ivyDeps = Agg(ivy"demo.runtime::library:4.0.0")
}
