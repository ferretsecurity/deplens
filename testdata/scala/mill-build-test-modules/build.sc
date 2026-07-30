import mill._, scalalib._

trait BaseModule extends ScalaModule {
  def scalaVersion = "2.13.14"
}

object main extends BaseModule {
  object test extends Tests with TestModule.ScalaTest {
    def ivyDeps = Agg(ivy"org.scalatest::scalatest:3.2.18")
  }
  object integrationTest extends Tests with TestModule.ScalaTest {
    def moduleDeps = super.moduleDeps ++ Seq(test)
    def ivyDeps = Agg(ivy"com.typesafe::config:1.4.3")
  }
}
