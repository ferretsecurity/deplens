import mill._, scalalib._

object app extends ScalaModule {
  def ivyDeps = Agg(ivy"demo.web::html:1.2.3")

  object test extends Tests {
    def ivyDeps = Agg(ivy"demo.test::checks:2.0.0")
  }
}
