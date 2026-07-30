import mill._, scalalib._

object main extends ScalaModule {
  def scalaVersion = "2.13.14"
  def ivyDeps = Agg(ivy"org.apache.spark::spark-sql:3.5.2;classifier=tests", ivy"com.example:without-logging:1.0.0".exclude("org.slf4j" -> "slf4j-api").excludeOrg("com.unwanted").excludeName("legacy-api"), ivy"com.lihaoyi::fansi:0.2.14".forceVersion())
}
