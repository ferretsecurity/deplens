(defproject example/profiles "0.1.0"
  :dependencies [[org.clojure/clojure "1.11.3"]]
  :plugins [[lein-ancient "0.7.0" :hooks false :middleware false]]
  :exclusions [commons-logging/commons-logging]
  :profiles {:dev ^{:pom-scope :provided} {:dependencies [[midje "1.10.10"]]}
             :test ^{:pom-scope :test} {:dependencies [[lambdaisland/kaocha "1.91.1392"]]
                                         :plugins [[lein-test-refresh "0.25.0"]]}}
  :repositories [["internal" "https://maven.example.test/releases"]])
