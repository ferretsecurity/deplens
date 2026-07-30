import $ivy.`com.lihaoyi::scalatags:0.12.0`
import $ivy.`org.thymeleaf:thymeleaf:3.1.1.RELEASE`
import $ivy.`com.lihaoyi::mill-contrib-bloop:$MILL_VERSION`
import $ivy.`com.lihaoyi::mill-contrib-versionfile:`
import mill._, scalalib._

trait BaseModule extends ScalaModule {
  def scalaVersion = "3.3.1"
}

object main extends BaseModule {
  def moduleDeps = Seq(util)
}

object util extends BaseModule {
  def ivyDeps = Agg(ivy"com.lihaoyi::mainargs:0.7.6")
}
