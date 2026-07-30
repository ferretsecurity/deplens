import mill._, scalalib._

object main extends ScalaModule {
  def scalaVersion = "3.3.1"
  def compileIvyDeps = Agg(ivy"jakarta.servlet:jakarta.servlet-api:6.1.0")
  def runIvyDeps = Agg(ivy"ch.qos.logback:logback-classic:1.5.7")
}
